package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"golang.org/x/mod/semver"
)

type Module struct {
	Source        string
	RepositoryURL string
	Namespace     string
	Name          string
	Provider      string
	Description   string
	LatestVersion string
	Downloads     int64
	Verified      bool
	PublishedAt   string
}

type ModuleDocPage struct {
	Module  Module
	Content string
	Source  string
	Website string
}

type ModuleVersion struct {
	Module      Module
	Version     string
	PublishedAt string
}

func (c Client) SearchModules(ctx context.Context, query string) ([]Module, error) {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return nil, fmt.Errorf("module search query is required")
	}

	if module, ok, err := c.exactModule(ctx, normalized, ""); ok || err != nil {
		if err != nil {
			return nil, err
		}
		return []Module{module}, nil
	}

	var modules []Module
	err := c.StreamSearchModules(ctx, normalized, func(page []Module) error {
		modules = append(modules, page...)
		return nil
	})
	return modules, err
}

func (c Client) StreamSearchModules(ctx context.Context, query string, yield func([]Module) error) error {
	normalized := strings.TrimSpace(query)
	if normalized == "" {
		return fmt.Errorf("module search query is required")
	}

	if module, ok, err := c.exactModule(ctx, normalized, ""); ok || err != nil {
		if err != nil {
			return err
		}
		return yield([]Module{module})
	}

	const pageSize = 100
	offset := 0
	for {
		modules, nextOffset, hasNext, err := c.searchModulesPage(ctx, normalized, offset, pageSize)
		if err != nil {
			return err
		}
		if len(modules) > 0 {
			sortModules(modules, normalized)
			if err := yield(modules); err != nil {
				return err
			}
		}
		if !hasNext {
			return nil
		}
		offset = nextOffset
	}
}

func (c Client) GetModuleDoc(ctx context.Context, moduleInput string, version string) (ModuleDocPage, error) {
	namespace, name, provider, err := parseModuleAddress(moduleInput)
	if err != nil {
		return ModuleDocPage{}, err
	}

	module, content, err := c.getModule(ctx, namespace, name, provider, strings.TrimSpace(version))
	if err != nil {
		return ModuleDocPage{}, err
	}
	return ModuleDocPage{
		Module:  module,
		Content: content,
		Source:  module.RepositoryURL,
		Website: moduleWebsiteURL(module),
	}, nil
}

func (c Client) ListModuleVersions(ctx context.Context, moduleInput string) ([]ModuleVersion, error) {
	namespace, name, provider, err := parseModuleAddress(moduleInput)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/v1/modules/%s/%s/%s/versions", c.BaseURL, url.PathEscape(namespace), url.PathEscape(name), url.PathEscape(provider))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var response moduleVersionsResponse
	if err := c.doJSON(req, &response); err != nil {
		return nil, err
	}

	module := Module{
		Source:    "registry.terraform.io/" + namespace + "/" + name + "/" + provider,
		Namespace: namespace,
		Name:      name,
		Provider:  provider,
	}
	if latest, _, err := c.getModule(ctx, namespace, name, provider, ""); err == nil {
		module = latest
	}

	var versions []ModuleVersion
	for _, item := range response.Modules {
		for _, version := range item.Versions {
			versions = append(versions, ModuleVersion{Module: module, Version: version.Version})
		}
	}

	sort.SliceStable(versions, func(i, j int) bool {
		left := semver.Canonical("v" + versions[i].Version)
		right := semver.Canonical("v" + versions[j].Version)
		if left != "" && right != "" && left != right {
			return semver.Compare(left, right) > 0
		}
		return strings.Compare(versions[i].Version, versions[j].Version) > 0
	})

	return versions, nil
}

func (c Client) exactModule(ctx context.Context, query string, version string) (Module, bool, error) {
	namespace, name, provider, err := parseModuleAddress(query)
	if err != nil {
		return Module{}, false, nil
	}

	module, _, err := c.getModule(ctx, namespace, name, provider, version)
	if err != nil {
		if isNotFound(err) {
			return Module{}, false, nil
		}
		return Module{}, false, err
	}
	return module, true, nil
}

func (c Client) getModule(ctx context.Context, namespace string, name string, provider string, version string) (Module, string, error) {
	parts := []string{
		c.BaseURL,
		"v1",
		"modules",
		url.PathEscape(namespace),
		url.PathEscape(name),
		url.PathEscape(provider),
	}
	if strings.TrimSpace(version) != "" {
		parts = append(parts, url.PathEscape(version))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.Join(parts, "/"), nil)
	if err != nil {
		return Module{}, "", err
	}

	var response moduleDetailResponse
	if err := c.doJSON(req, &response); err != nil {
		return Module{}, "", err
	}

	module := moduleFromDetail(response)
	return module, response.Root.Readme, nil
}

func (c Client) searchModulesPage(ctx context.Context, query string, offset int, limit int) ([]Module, int, bool, error) {
	endpoint, err := url.Parse(c.BaseURL + "/v1/modules/search")
	if err != nil {
		return nil, 0, false, err
	}
	params := endpoint.Query()
	params.Set("q", query)
	params.Set("limit", fmt.Sprintf("%d", limit))
	params.Set("offset", fmt.Sprintf("%d", offset))
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, 0, false, err
	}

	var response moduleSearchResponse
	if err := c.doJSON(req, &response); err != nil {
		return nil, 0, false, err
	}

	modules := make([]Module, 0, len(response.Modules))
	for _, item := range response.Modules {
		modules = append(modules, moduleFromSearch(item))
	}

	return modules, response.Meta.NextOffset, response.Meta.NextURL != "", nil
}

func parseModuleAddress(input string) (string, string, string, error) {
	source := strings.TrimSpace(input)
	source = strings.TrimPrefix(source, "registry.terraform.io/")
	parts := strings.Split(source, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", "", "", fmt.Errorf("module source must be namespace/name/provider")
	}
	return parts[0], parts[1], parts[2], nil
}

func moduleFromSearch(item moduleSearchItem) Module {
	return Module{
		Source:        "registry.terraform.io/" + item.Namespace + "/" + item.Name + "/" + item.Provider,
		RepositoryURL: item.Source,
		Namespace:     item.Namespace,
		Name:          item.Name,
		Provider:      item.Provider,
		Description:   item.Description,
		LatestVersion: item.Version,
		Downloads:     item.Downloads,
		Verified:      item.Verified,
		PublishedAt:   item.PublishedAt,
	}
}

func moduleFromDetail(response moduleDetailResponse) Module {
	return Module{
		Source:        "registry.terraform.io/" + response.Namespace + "/" + response.Name + "/" + response.Provider,
		RepositoryURL: response.Source,
		Namespace:     response.Namespace,
		Name:          response.Name,
		Provider:      response.Provider,
		Description:   response.Description,
		LatestVersion: response.Version,
		Downloads:     response.Downloads,
		Verified:      response.Verified,
		PublishedAt:   response.PublishedAt,
	}
}

func sortModules(modules []Module, query string) {
	sort.SliceStable(modules, func(i, j int) bool {
		left := moduleScore(modules[i], query)
		right := moduleScore(modules[j], query)
		if left != right {
			return left > right
		}
		if modules[i].Downloads != modules[j].Downloads {
			return modules[i].Downloads > modules[j].Downloads
		}
		return modules[i].Source < modules[j].Source
	})
}

func moduleScore(module Module, query string) int {
	score := 0
	shortSource := strings.TrimPrefix(module.Source, "registry.terraform.io/")
	if module.Name == query {
		score += 1000
	}
	if shortSource == query {
		score += 2000
	}
	if module.Verified {
		score += 250
	}
	return score
}

func moduleWebsiteURL(module Module) string {
	source := strings.TrimPrefix(module.Source, "registry.terraform.io/")
	parts := strings.Split(source, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}

	version := module.LatestVersion
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}

	return fmt.Sprintf("https://registry.terraform.io/modules/%s/%s/%s/%s", parts[0], parts[1], parts[2], version)
}

type moduleSearchResponse struct {
	Meta struct {
		NextOffset int    `json:"next_offset"`
		NextURL    string `json:"next_url"`
	} `json:"meta"`
	Modules []moduleSearchItem `json:"modules"`
}

type moduleSearchItem struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Source      string `json:"source"`
	PublishedAt string `json:"published_at"`
	Downloads   int64  `json:"downloads"`
	Verified    bool   `json:"verified"`
}

type moduleDetailResponse struct {
	Namespace   string `json:"namespace"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Source      string `json:"source"`
	PublishedAt string `json:"published_at"`
	Downloads   int64  `json:"downloads"`
	Verified    bool   `json:"verified"`
	Root        struct {
		Readme string `json:"readme"`
	} `json:"root"`
}

type moduleVersionsResponse struct {
	Modules []struct {
		Versions []struct {
			Version string `json:"version"`
		} `json:"versions"`
	} `json:"modules"`
}
