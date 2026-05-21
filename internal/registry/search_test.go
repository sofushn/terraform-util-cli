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
