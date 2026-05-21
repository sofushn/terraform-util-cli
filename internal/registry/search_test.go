package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchProvidersRanksAndHydratesVersions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/providers":
			if got := r.URL.Query().Get("filter[name]"); got != "aws" {
				t.Fatalf("unexpected filter[name]: %q", got)
			}
			w.Write([]byte(`{
				"data": [
					{"attributes": {"full-name": "aaronfeng/aws", "namespace": "aaronfeng", "name": "aws", "alias": "aws", "description": "", "downloads": 100, "tier": "community"}},
					{"attributes": {"full-name": "hashicorp/aws", "namespace": "hashicorp", "name": "aws", "alias": "aws", "description": "", "downloads": 500, "tier": "official"}}
				]
			}`))
		case "/v1/providers/hashicorp/aws":
			w.Write([]byte(`{"namespace":"hashicorp","name":"aws","alias":"aws","version":"6.46.0","description":"AWS","downloads":500,"tier":"official"}`))
		case "/v1/providers/aaronfeng/aws":
			w.Write([]byte(`{"namespace":"aaronfeng","name":"aws","alias":"aws","version":"1.0.0","description":"AWS","downloads":100,"tier":"community"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	providers, err := client.SearchProviders(context.Background(), "aws")
	if err != nil {
		t.Fatalf("search providers: %v", err)
	}

	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
	if providers[0].Source != "registry.terraform.io/hashicorp/aws" {
		t.Fatalf("expected hashicorp/aws first, got %#v", providers[0])
	}
	if providers[0].LatestVersion != "6.46.0" {
		t.Fatalf("expected hydrated version, got %#v", providers[0])
	}
	if !providers[0].Verified {
		t.Fatalf("expected official provider to be verified")
	}
}

func TestSearchProvidersExactSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/providers/hashicorp/aws" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"namespace":"hashicorp","name":"aws","alias":"aws","version":"6.46.0","description":"AWS","downloads":500,"tier":"official"}`))
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	providers, err := client.SearchProviders(context.Background(), "registry.terraform.io/hashicorp/aws")
	if err != nil {
		t.Fatalf("search providers: %v", err)
	}
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Source != "registry.terraform.io/hashicorp/aws" || providers[0].LatestVersion != "6.46.0" {
		t.Fatalf("unexpected provider: %#v", providers[0])
	}
}

func TestResolveProviderShortNameUsesMostDownloads(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/providers":
			w.Write([]byte(`{
				"data": [
					{"attributes": {"full-name": "hashicorp/example", "namespace": "hashicorp", "name": "example", "alias": "example", "description": "", "downloads": 100, "tier": "official"}},
					{"attributes": {"full-name": "popular/example", "namespace": "popular", "name": "example", "alias": "example", "description": "", "downloads": 999, "tier": "community"}}
				]
			}`))
		case "/v1/providers/popular/example":
			w.Write([]byte(`{"namespace":"popular","name":"example","alias":"example","version":"1.2.3","description":"Example","downloads":999,"tier":"community"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	provider, err := client.ResolveProvider(context.Background(), "example")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider.Namespace != "popular" || provider.Name != "example" {
		t.Fatalf("expected popular/example, got %#v", provider)
	}
	if provider.LatestVersion != "1.2.3" {
		t.Fatalf("expected hydrated version, got %#v", provider)
	}
}

func TestResolveProviderAggregatesSearchPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/providers":
			if got := r.URL.Query().Get("page[size]"); got != "100" {
				t.Fatalf("unexpected page size: %q", got)
			}
			switch r.URL.Query().Get("page[number]") {
			case "1":
				w.Write([]byte(providerSearchResponseJSON("other", 100)))
			case "2":
				w.Write([]byte(`{
					"data": [
						{"attributes": {"full-name": "popular/example", "namespace": "popular", "name": "example", "alias": "example", "description": "", "downloads": 999, "tier": "community"}}
					]
				}`))
			default:
				w.Write([]byte(`{"data":[]}`))
			}
		case "/v1/providers/popular/example":
			w.Write([]byte(`{"namespace":"popular","name":"example","alias":"example","version":"1.2.3","description":"Example","downloads":999,"tier":"community"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	provider, err := client.ResolveProvider(context.Background(), "example")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider.Source != "registry.terraform.io/popular/example" {
		t.Fatalf("unexpected provider: %#v", provider)
	}
}

func TestResolveProviderNamespacedUsesExactProvider(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/providers/hashicorp/aws" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{"namespace":"hashicorp","name":"aws","alias":"aws","version":"6.46.0","description":"AWS","downloads":500,"tier":"official"}`))
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	provider, err := client.ResolveProvider(context.Background(), "hashicorp/aws")
	if err != nil {
		t.Fatalf("resolve provider: %v", err)
	}
	if provider.Source != "registry.terraform.io/hashicorp/aws" {
		t.Fatalf("unexpected provider: %#v", provider)
	}
}

func TestListProviderDocsReturnsSupportedDocs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/providers/hashicorp/aws":
			w.Write([]byte(`{
				"included": [
					{"type":"provider-versions","id":"97303","attributes":{"version":"6.46.0","tag":"v6.46.0"}}
				]
			}`))
		case "/v2/provider-docs":
			if got := r.URL.Query().Get("filter[provider-version]"); got != "97303" {
				t.Fatalf("unexpected provider version: %q", got)
			}
			if got := r.URL.Query().Get("filter[language]"); got != "hcl" {
				t.Fatalf("unexpected language: %q", got)
			}
			category := r.URL.Query().Get("filter[category]")
			page := r.URL.Query().Get("page[number]")
			switch {
			case category == "resources" && page == "1":
				w.Write([]byte(providerDocsResponseJSON("resources", 100, "resource")))
			case category == "resources" && page == "2":
				w.Write([]byte(providerDocsResponseJSON("resources", 1, "tail")))
			case category == "data-sources" && page == "1":
				w.Write([]byte(`{"data":[{"type":"provider-docs","id":"data-1","attributes":{"category":"data-sources","language":"hcl","path":"website/docs/d/ami.html.markdown","slug":"ami","title":"ami"}}]}`))
			default:
				w.Write([]byte(`{"data":[]}`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	items, err := client.ListProviderDocs(context.Background(), Provider{
		Namespace:     "hashicorp",
		Name:          "aws",
		LatestVersion: "6.46.0",
	})
	if err != nil {
		t.Fatalf("list provider docs: %v", err)
	}

	if len(items) != 102 {
		t.Fatalf("expected 102 supported docs, got %d", len(items))
	}
	if items[0].Kind != "resource" || items[0].Name != "aws_resource_000" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[100].Kind != "resource" || items[100].Name != "aws_tail_000" {
		t.Fatalf("expected second page resource, got %#v", items[100])
	}
	if items[101].Kind != "data" || items[101].Name != "aws_ami" {
		t.Fatalf("unexpected data item: %#v", items[101])
	}
}

func TestGetProviderDocFetchesContentAndSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/providers/hashicorp/aws":
			w.Write([]byte(`{
				"included": [
					{"type":"provider-versions","id":"97303","attributes":{"version":"6.46.0","tag":"v6.46.0"}}
				]
			}`))
		case "/v2/provider-docs":
			if got := r.URL.Query().Get("filter[category]"); got != "resources" {
				t.Fatalf("unexpected category: %q", got)
			}
			if got := r.URL.Query().Get("filter[slug]"); got != "aws_vpc,vpc" {
				t.Fatalf("unexpected slug candidates: %q", got)
			}
			w.Write([]byte(`{
				"data": [
					{"type":"provider-docs","id":"123","attributes":{"category":"resources","language":"hcl","path":"website/docs/r/vpc.html.markdown","slug":"vpc","title":"vpc"}}
				]
			}`))
		case "/v2/provider-docs/123":
			w.Write([]byte(`{
				"data": {"type":"provider-docs","id":"123","attributes":{"category":"resources","content":"---\npage_title: AWS VPC\n---\n\n# Resource: aws_vpc\n","language":"hcl","path":"website/docs/r/vpc.html.markdown","slug":"vpc","title":"vpc"}}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	page, err := client.GetProviderDoc(context.Background(), Provider{
		Namespace:     "hashicorp",
		Name:          "aws",
		LatestVersion: "6.46.0",
		RepositoryURL: "https://github.com/hashicorp/terraform-provider-aws",
	}, "resource", "aws_vpc")
	if err != nil {
		t.Fatalf("get provider doc: %v", err)
	}

	if page.Content != "# Resource: aws_vpc" {
		t.Fatalf("unexpected content: %q", page.Content)
	}
	if page.Source != "https://github.com/hashicorp/terraform-provider-aws/blob/v6.46.0/website/docs/r/vpc.html.markdown" {
		t.Fatalf("unexpected source: %q", page.Source)
	}
	if page.Website != "https://registry.terraform.io/providers/hashicorp/aws/6.46.0/docs/resources/vpc" {
		t.Fatalf("unexpected website: %q", page.Website)
	}
}

func providerDocsResponseJSON(category string, count int, prefix string) string {
	var b strings.Builder
	b.WriteString(`{"data":[`)
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		slug := fmt.Sprintf("%s_%03d", prefix, i)
		path := fmt.Sprintf("website/docs/r/%s.html.markdown", slug)
		fmt.Fprintf(&b, `{"type":"provider-docs","id":"%s-%d","attributes":{"category":%q,"language":"hcl","path":%q,"slug":%q,"title":%q}}`, prefix, i, category, path, slug, slug)
	}
	b.WriteString(`]}`)
	return b.String()
}

func providerSearchResponseJSON(prefix string, count int) string {
	var b strings.Builder
	b.WriteString(`{"data":[`)
	for i := 0; i < count; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		name := fmt.Sprintf("%s-%03d", prefix, i)
		fullName := "namespace/" + name
		fmt.Fprintf(&b, `{"attributes":{"full-name":%q,"namespace":"namespace","name":%q,"alias":%q,"description":"","downloads":1,"tier":"community"}}`, fullName, name, name)
	}
	b.WriteString(`]}`)
	return b.String()
}
