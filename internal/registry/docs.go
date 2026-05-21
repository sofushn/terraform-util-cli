package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
)

type DocItem struct {
	Kind  string
	Name  string
	Title string
	Path  string
	Slug  string
}

type DocPage struct {
	Kind    string
	Name    string
	Title   string
	Path    string
	Content string
	Source  string
}

func (c Client) ListProviderDocs(ctx context.Context, provider Provider) ([]DocItem, error) {
	versionID, _, err := c.providerVersionID(ctx, provider.Namespace, provider.Name, provider.LatestVersion)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/v2/provider-versions/%s?include=provider-docs", c.BaseURL, url.PathEscape(versionID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}

	var response v2ProviderVersionResponse
	if err := c.doJSON(req, &response); err != nil {
		return nil, err
	}

	items := make([]DocItem, 0, len(response.Included))
	for _, doc := range response.Included {
		item, ok := docItemFromAttributes(doc.Attributes)
		if ok {
			items = append(items, item)
		}
	}

	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return docKindRank(items[i].Kind) < docKindRank(items[j].Kind)
		}
		return items[i].Name < items[j].Name
	})

	return items, nil
}

func (c Client) GetProviderDoc(ctx context.Context, provider Provider, kind string, name string) (DocPage, error) {
	versionID, versionTag, err := c.providerVersionID(ctx, provider.Namespace, provider.Name, provider.LatestVersion)
	if err != nil {
		return DocPage{}, err
	}

	doc, err := c.findProviderDoc(ctx, versionID, provider.Name, kind, name)
	if err != nil {
		return DocPage{}, err
	}

	page, err := c.getProviderDoc(ctx, doc.ID)
	if err != nil {
		return DocPage{}, err
	}

	item, ok := docItemFromAttributes(page.Data.Attributes)
	if !ok {
		return DocPage{}, fmt.Errorf("provider doc %q is not a supported docs type", name)
	}

	source := providerDocSource(provider.RepositoryURL, versionTag, item.Path)
	return DocPage{
		Kind:    item.Kind,
		Name:    item.Name,
		Title:   item.Title,
		Path:    item.Path,
		Content: stripFrontMatter(page.Data.Attributes.Content),
		Source:  source,
	}, nil
}

func (c Client) findProviderDoc(ctx context.Context, versionID string, providerName string, kind string, name string) (v2ProviderDocData, error) {
	category, err := docsCategory(kind)
	if err != nil {
		return v2ProviderDocData{}, err
	}

	slugs := candidateSlugs(providerName, name)
	endpoint, err := url.Parse(c.BaseURL + "/v2/provider-docs")
	if err != nil {
		return v2ProviderDocData{}, err
	}
	params := endpoint.Query()
	params.Set("filter[provider-version]", versionID)
	params.Set("filter[category]", category)
	params.Set("filter[slug]", strings.Join(slugs, ","))
	params.Set("filter[language]", "hcl")
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return v2ProviderDocData{}, err
	}

	var response v2ProviderDocsResponse
	if err := c.doJSON(req, &response); err != nil {
		return v2ProviderDocData{}, err
	}
	if len(response.Data) == 0 {
		return v2ProviderDocData{}, fmt.Errorf("provider doc %q not found", kind+"/"+name)
	}

	for _, candidate := range slugs {
		for _, doc := range response.Data {
			if doc.Attributes.Slug == candidate {
				return doc, nil
			}
		}
	}

	return response.Data[0], nil
}

func (c Client) getProviderDoc(ctx context.Context, id string) (v2ProviderDocResponse, error) {
	endpoint := fmt.Sprintf("%s/v2/provider-docs/%s", c.BaseURL, url.PathEscape(id))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return v2ProviderDocResponse{}, err
	}

	var response v2ProviderDocResponse
	if err := c.doJSON(req, &response); err != nil {
		return v2ProviderDocResponse{}, err
	}
	return response, nil
}

func (c Client) providerVersionID(ctx context.Context, namespace string, name string, version string) (string, string, error) {
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}

	include := "latest-version"
	if version != "latest" {
		include = "provider-versions"
	}

	endpoint, err := url.Parse(fmt.Sprintf("%s/v2/providers/%s/%s", c.BaseURL, url.PathEscape(namespace), url.PathEscape(name)))
	if err != nil {
		return "", "", err
	}
	params := endpoint.Query()
	params.Set("include", include)
	endpoint.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return "", "", err
	}

	var response v2ProviderWithVersionsResponse
	if err := c.doJSON(req, &response); err != nil {
		return "", "", err
	}

	for _, item := range response.Included {
		if item.Type != "provider-versions" {
			continue
		}
		if version == "latest" || item.Attributes.Version == version {
			return item.ID, item.Attributes.Tag, nil
		}
	}

	return "", "", fmt.Errorf("provider version %s/%s %q not found", namespace, name, version)
}

func docItemFromAttributes(attrs v2ProviderDocAttributes) (DocItem, bool) {
	kind, ok := docsKind(attrs.Category)
	if !ok || attrs.Language != "hcl" {
		return DocItem{}, false
	}

	return DocItem{
		Kind:  kind,
		Name:  canonicalDocName(kind, attrs.Title, attrs.Slug),
		Title: attrs.Title,
		Path:  attrs.Path,
		Slug:  attrs.Slug,
	}, true
}

func docsCategory(kind string) (string, error) {
	switch kind {
	case "resource":
		return "resources", nil
	case "data":
		return "data-sources", nil
	case "function":
		return "functions", nil
	default:
		return "", fmt.Errorf("unsupported docs kind %q", kind)
	}
}

func docsKind(category string) (string, bool) {
	switch category {
	case "resources":
		return "resource", true
	case "data-sources":
		return "data", true
	case "functions":
		return "function", true
	default:
		return "", false
	}
}

func canonicalDocName(kind string, title string, slug string) string {
	if strings.TrimSpace(title) != "" {
		return title
	}
	return slug
}

func candidateSlugs(providerName string, name string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			return
		}
		seen[value] = true
		out = append(out, value)
	}

	add(name)
	add(strings.TrimPrefix(name, providerName+"_"))
	return out
}

func providerDocSource(repositoryURL string, versionTag string, path string) string {
	if repositoryURL == "" || versionTag == "" || path == "" {
		return ""
	}
	return strings.TrimRight(repositoryURL, "/") + "/blob/" + versionTag + "/" + path
}

func stripFrontMatter(content string) string {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---\n") {
		return content
	}

	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---")
	if end == -1 {
		return content
	}

	return strings.TrimSpace(rest[end+len("\n---"):])
}

func docKindRank(kind string) int {
	switch kind {
	case "resource":
		return 0
	case "data":
		return 1
	case "function":
		return 2
	default:
		return 3
	}
}

type v2ProviderVersionResponse struct {
	Included []v2ProviderDocData `json:"included"`
}

type v2ProviderDocsResponse struct {
	Data []v2ProviderDocData `json:"data"`
}

type v2ProviderDocResponse struct {
	Data v2ProviderDocData `json:"data"`
}

type v2ProviderDocData struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`
	Attributes v2ProviderDocAttributes `json:"attributes"`
}

type v2ProviderDocAttributes struct {
	Category string `json:"category"`
	Content  string `json:"content"`
	Language string `json:"language"`
	Path     string `json:"path"`
	Slug     string `json:"slug"`
	Title    string `json:"title"`
}

type v2ProviderWithVersionsResponse struct {
	Included []struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Attributes struct {
			Tag     string `json:"tag"`
			Version string `json:"version"`
		} `json:"attributes"`
	} `json:"included"`
}
