package registry

import (
	"context"
	"net/http"
	"net/http/httptest"
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
		case "/v2/provider-versions/97303":
			if got := r.URL.Query().Get("include"); got != "provider-docs" {
				t.Fatalf("unexpected include: %q", got)
			}
			w.Write([]byte(`{
				"included": [
					{"type":"provider-docs","id":"1","attributes":{"category":"resources","language":"hcl","path":"website/docs/r/vpc.html.markdown","slug":"vpc","title":"vpc"}},
					{"type":"provider-docs","id":"2","attributes":{"category":"data-sources","language":"hcl","path":"website/docs/d/ami.html.markdown","slug":"ami","title":"ami"}},
					{"type":"provider-docs","id":"3","attributes":{"category":"guides","language":"hcl","path":"website/docs/guides/example.html.markdown","slug":"example","title":"Example"}}
				]
			}`))
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

	if len(items) != 2 {
		t.Fatalf("expected 2 supported docs, got %#v", items)
	}
	if items[0].Kind != "resource" || items[0].Name != "aws_vpc" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Kind != "data" || items[1].Name != "aws_ami" {
		t.Fatalf("unexpected second item: %#v", items[1])
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
}
