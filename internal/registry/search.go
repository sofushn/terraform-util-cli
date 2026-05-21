package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const officialRegistryURL = "https://registry.terraform.io"

type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

type Provider struct {
	Source        string
	RepositoryURL string
	Namespace     string
	Name          string
	DisplayName   string
	Description   string
	LatestVersion string
	Downloads     int64
	Verified      bool
	Tier          string
}

func NewClient() Client {
	return NewClientForBaseURL(officialRegistryURL)
}

func NewClientForBaseURL(baseURL string) Client {
	return Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c Client) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return nil, fmt.Errorf("provider search query is required")
	}

	if provider, ok, err := c.exactProvider(ctx, normalized); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return []Provider{provider}, nil
	}

	providers, err := c.searchByName(ctx, normalized)
	if err != nil {
		return nil, err
	}

	sortProviders(providers, normalized)

	c.hydrateProviderVersions(ctx, providers)

	return providers, nil
}

func (c Client) StreamSearchProviders(ctx context.Context, query string, yield func([]Provider) error) error {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return fmt.Errorf("provider search query is required")
	}

	if provider, ok, err := c.exactProvider(ctx, normalized); ok || err != nil {
		if err != nil {
			return err
		}
		return yield([]Provider{provider})
	}

	return c.searchByNamePages(ctx, normalized, func(providers []Provider) error {
		sortProviders(providers, normalized)
		c.hydrateProviderVersions(ctx, providers)
		return yield(providers)
	})
}

func (c Client) ResolveProvider(ctx context.Context, query string) (Provider, error) {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return Provider{}, fmt.Errorf("provider is required")
	}

	if provider, ok, err := c.exactProvider(ctx, normalized); ok || err != nil {
		if err != nil {
			return Provider{}, err
		}
		return provider, nil
	}

	if strings.Contains(normalized, "/") {
		return Provider{}, fmt.Errorf("provider %q not found", query)
	}

	providers, err := c.searchByName(ctx, normalized)
	if err != nil {
		return Provider{}, err
	}

	var selected *Provider
	for i := range providers {
		if providers[i].Name != normalized {
			continue
		}
		if selected == nil || providers[i].Downloads > selected.Downloads {
			selected = &providers[i]
		}
	}
	if selected == nil {
		return Provider{}, fmt.Errorf("provider %q not found", query)
	}

	provider := *selected
	resolvedProviders := []Provider{provider}
	c.hydrateProviderVersions(ctx, resolvedProviders)
	return resolvedProviders[0], nil
}

func (c Client) exactProvider(ctx context.Context, query string) (Provider, bool, error) {
	parts := strings.Split(query, "/")
	if len(parts) == 3 && parts[0] == "registry.terraform.io" {
		parts = parts[1:]
	}
	if len(parts) != 2 {
		return Provider{}, false, nil
	}

	provider, err := c.getProvider(ctx, parts[0], parts[1])
	if err != nil {
		if isNotFound(err) {
			return Provider{}, false, nil
		}
		return Provider{}, false, err
	}
	return provider, true, nil
}

func (c Client) searchByName(ctx context.Context, query string) ([]Provider, error) {
	var providers []Provider
	err := c.searchByNamePages(ctx, query, func(page []Provider) error {
		providers = append(providers, page...)
		return nil
	})
	return providers, err
}

func (c Client) searchByNamePages(ctx context.Context, query string, yield func([]Provider) error) error {
	const pageSize = 100

	for pageNumber := 1; ; pageNumber++ {
		providers, page, err := c.searchByNamePage(ctx, query, pageNumber, pageSize)
		if err != nil {
			return err
		}

		if len(providers) > 0 {
			if err := yield(providers); err != nil {
				return err
			}
		}

		if page.isLast(pageNumber, pageSize) {
			return nil
		}
	}
}

func (c Client) searchByNamePage(ctx context.Context, query string, pageNumber int, pageSize int) ([]Provider, pagination, error) {
	endpoint, err := url.Parse(c.BaseURL + "/v2/providers")
	if err != nil {
		return nil, pagination{}, err
	}
	params := endpoint.Query()
	params.Set("filter[name]", query)
	params.Set("page[size]", fmt.Sprintf("%d", pageSize))
	params.Set("page[number]", fmt.Sprintf("%d", pageNumber))
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, pagination{}, err
	}

	var response v2ProvidersResponse
	if err := c.doJSON(req, &response); err != nil {
		return nil, pagination{}, err
	}

	providers := make([]Provider, 0, len(response.Data))
	for _, item := range response.Data {
		attrs := item.Attributes
		providers = append(providers, Provider{
			Source:        "registry.terraform.io/" + attrs.FullName,
			RepositoryURL: attrs.Source,
			Namespace:     attrs.Namespace,
			Name:          attrs.Name,
			DisplayName:   displayName(attrs.Alias, attrs.Name),
			Description:   attrs.Description,
			Downloads:     attrs.Downloads,
			Verified:      isVerified(attrs.Tier),
			Tier:          attrs.Tier,
		})
	}

	return providers, pagination{totalPages: response.Meta.Pagination.TotalPages, itemCount: len(response.Data)}, nil
}

func (c Client) getProvider(ctx context.Context, namespace string, name string) (Provider, error) {
	endpoint := fmt.Sprintf("%s/v1/providers/%s/%s", c.BaseURL, url.PathEscape(namespace), url.PathEscape(name))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return Provider{}, err
	}

	var response v1ProviderResponse
	if err := c.doJSON(req, &response); err != nil {
		return Provider{}, err
	}

	return Provider{
		Source:        "registry.terraform.io/" + response.Namespace + "/" + response.Name,
		RepositoryURL: response.Source,
		Namespace:     response.Namespace,
		Name:          response.Name,
		DisplayName:   displayName(response.Alias, response.Name),
		Description:   response.Description,
		LatestVersion: response.Version,
		Downloads:     response.Downloads,
		Verified:      isVerified(response.Tier),
		Tier:          response.Tier,
	}, nil
}

func (c Client) latestVersion(ctx context.Context, namespace string, name string) (string, error) {
	provider, err := c.getProvider(ctx, namespace, name)
	if err != nil {
		return "", err
	}
	return provider.LatestVersion, nil
}

func (c Client) hydrateProviderVersions(ctx context.Context, providers []Provider) {
	var wg sync.WaitGroup
	for i := range providers {
		if providers[i].LatestVersion != "" {
			continue
		}

		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			version, err := c.latestVersion(ctx, providers[i].Namespace, providers[i].Name)
			if err == nil {
				providers[i].LatestVersion = version
			}
		}(i)
	}
	wg.Wait()
}

func (c Client) doJSON(req *http.Request, target any) error {
	client := c.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return notFoundError{url: req.URL.String()}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("registry request failed: %s", resp.Status)
	}

	return json.NewDecoder(resp.Body).Decode(target)
}

func sortProviders(providers []Provider, query string) {
	sort.SliceStable(providers, func(i, j int) bool {
		left := providerScore(providers[i], query)
		right := providerScore(providers[j], query)
		if left != right {
			return left > right
		}
		if providers[i].Downloads != providers[j].Downloads {
			return providers[i].Downloads > providers[j].Downloads
		}
		return providers[i].Source < providers[j].Source
	})
}

func providerScore(provider Provider, query string) int {
	score := 0
	if provider.Name == query {
		score += 1000
	}
	if provider.Namespace+"/"+provider.Name == query {
		score += 2000
	}
	if provider.Namespace == "hashicorp" {
		score += 500
	}
	if provider.Verified {
		score += 250
	}
	return score
}

func displayName(alias string, name string) string {
	if strings.TrimSpace(alias) != "" {
		return alias
	}
	return name
}

func isVerified(tier string) bool {
	return tier == "official" || tier == "partner"
}

type pagination struct {
	totalPages int
	itemCount  int
}

func (p pagination) isLast(pageNumber int, pageSize int) bool {
	if p.totalPages > 0 {
		return pageNumber >= p.totalPages
	}
	return p.itemCount < pageSize
}

type notFoundError struct {
	url string
}

func (e notFoundError) Error() string {
	return "registry resource not found: " + e.url
}

func isNotFound(err error) bool {
	_, ok := err.(notFoundError)
	return ok
}

type v1ProviderResponse struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Alias       string `json:"alias"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Downloads   int64  `json:"downloads"`
	Source      string `json:"source"`
	Tier        string `json:"tier"`
}

type v2ProvidersResponse struct {
	Data []struct {
		Attributes struct {
			Alias       string `json:"alias"`
			Description string `json:"description"`
			Downloads   int64  `json:"downloads"`
			FullName    string `json:"full-name"`
			Name        string `json:"name"`
			Namespace   string `json:"namespace"`
			Source      string `json:"source"`
			Tier        string `json:"tier"`
		} `json:"attributes"`
	} `json:"data"`
	Meta paginationMeta `json:"meta"`
}

type paginationMeta struct {
	Pagination struct {
		TotalPages int `json:"total-pages"`
	} `json:"pagination"`
}
