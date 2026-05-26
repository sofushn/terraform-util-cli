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

func TestSearchProvidersReturnsAllResults(t *testing.T) {
	const providerCount = 16

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/providers":
			fmt.Fprint(w, providerSearchResponseJSON("aws", providerCount))
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

	if len(providers) != providerCount {
		t.Fatalf("expected %d providers, got %d", providerCount, len(providers))
	}
}

func TestStreamSearchProvidersYieldsPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/v2/providers":
			switch r.URL.Query().Get("page[number]") {
			case "1":
				fmt.Fprint(w, providerSearchResponseJSONWithTotalPages("aws", 1, 2))
			case "2":
				fmt.Fprint(w, providerSearchResponseJSONWithTotalPages("aws-tail", 1, 2))
			default:
				t.Fatalf("unexpected page number: %s", r.URL.Query().Get("page[number]"))
			}
		case strings.HasPrefix(r.URL.Path, "/v1/providers/namespace/"):
			parts := strings.Split(r.URL.Path, "/")
			name := parts[len(parts)-1]
			fmt.Fprintf(w, `{"namespace":"namespace","name":%q,"alias":%q,"version":"1.0.0","description":"","downloads":1,"tier":"community"}`, name, name)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	var pageLengths []int
	err := client.StreamSearchProviders(context.Background(), "aws", func(providers []Provider) error {
		pageLengths = append(pageLengths, len(providers))
		return nil
	})
	if err != nil {
		t.Fatalf("stream search providers: %v", err)
	}

	if fmt.Sprint(pageLengths) != "[1 1]" {
		t.Fatalf("unexpected page lengths: %v", pageLengths)
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

func TestStreamSearchModulesYieldsPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/modules/search":
			switch r.URL.Query().Get("offset") {
			case "0":
				w.Write([]byte(`{
					"meta": {"next_offset": 100, "next_url": "/v1/modules/search?q=vpc&offset=100&limit=100"},
					"modules": [
						{"namespace":"terraform-aws-modules","name":"vpc","provider":"aws","version":"6.6.1","description":"VPC","source":"https://github.com/terraform-aws-modules/terraform-aws-vpc","downloads":20,"verified":false}
					]
				}`))
			case "100":
				w.Write([]byte(`{
					"meta": {},
					"modules": [
						{"namespace":"aws-ia","name":"vpc","provider":"aws","version":"4.7.3","description":"VPC","source":"https://github.com/aws-ia/terraform-aws-vpc","downloads":30,"verified":true}
					]
				}`))
			default:
				t.Fatalf("unexpected offset: %s", r.URL.Query().Get("offset"))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	var pages [][]Module
	err := client.StreamSearchModules(context.Background(), "vpc", func(modules []Module) error {
		pages = append(pages, modules)
		return nil
	})
	if err != nil {
		t.Fatalf("stream search modules: %v", err)
	}
	if len(pages) != 2 || pages[0][0].Source != "registry.terraform.io/terraform-aws-modules/vpc/aws" || !pages[1][0].Verified {
		t.Fatalf("unexpected pages: %#v", pages)
	}
}

func TestGetModuleDocFetchesVersionedReadme(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/modules/terraform-aws-modules/vpc/aws/6.6.1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Write([]byte(`{
			"namespace":"terraform-aws-modules",
			"name":"vpc",
			"provider":"aws",
			"version":"6.6.1",
			"description":"VPC",
			"source":"https://github.com/terraform-aws-modules/terraform-aws-vpc",
			"downloads":20,
			"verified":false,
			"published_at":"2026-04-02T20:22:11Z",
			"root":{"readme":"# AWS VPC Terraform module"}
		}`))
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	page, err := client.GetModuleDoc(context.Background(), "registry.terraform.io/terraform-aws-modules/vpc/aws", "6.6.1")
	if err != nil {
		t.Fatalf("get module doc: %v", err)
	}
	if page.Module.Source != "registry.terraform.io/terraform-aws-modules/vpc/aws" || page.Content != "# AWS VPC Terraform module" {
		t.Fatalf("unexpected module page: %#v", page)
	}
	if page.Website != "https://registry.terraform.io/modules/terraform-aws-modules/vpc/aws/6.6.1" {
		t.Fatalf("unexpected website: %s", page.Website)
	}
}

func TestListModuleVersionsSortsNewestFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/modules/terraform-aws-modules/vpc/aws/versions":
			w.Write([]byte(`{"modules":[{"versions":[{"version":"1.0.0"},{"version":"6.6.1"},{"version":"6.5.0"}]}]}`))
		case "/v1/modules/terraform-aws-modules/vpc/aws":
			w.Write([]byte(`{
				"namespace":"terraform-aws-modules",
				"name":"vpc",
				"provider":"aws",
				"version":"6.6.1",
				"source":"https://github.com/terraform-aws-modules/terraform-aws-vpc",
				"root":{"readme":""}
			}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	versions, err := client.ListModuleVersions(context.Background(), "terraform-aws-modules/vpc/aws")
	if err != nil {
		t.Fatalf("list module versions: %v", err)
	}
	if got := fmt.Sprintf("%s,%s,%s", versions[0].Version, versions[1].Version, versions[2].Version); got != "6.6.1,6.5.0,1.0.0" {
		t.Fatalf("unexpected version order: %s", got)
	}
	if versions[0].Module.Source != "registry.terraform.io/terraform-aws-modules/vpc/aws" {
		t.Fatalf("unexpected module on versions: %#v", versions[0].Module)
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
			case category == "overview" && page == "1":
				w.Write([]byte(`{"data":[{"type":"provider-docs","id":"overview-1","attributes":{"category":"overview","language":"hcl","path":"website/docs/index.html.markdown","slug":"index","title":"overview"}}]}`))
			case category == "guides" && page == "1":
				w.Write([]byte(`{"data":[{"type":"provider-docs","id":"guide-1","attributes":{"category":"guides","language":"hcl","path":"website/docs/guides/custom-service-endpoints.html.markdown","slug":"custom-service-endpoints","title":"Terraform AWS Provider Custom Service Endpoint Configuration"}}]}`))
			case category == "resources" && page == "1":
				w.Write([]byte(providerDocsResponseJSON("resources", 100, "resource")))
			case category == "resources" && page == "2":
				w.Write([]byte(providerDocsResponseJSON("resources", 1, "tail")))
			case category == "data-sources" && page == "1":
				w.Write([]byte(`{"data":[{"type":"provider-docs","id":"data-1","attributes":{"category":"data-sources","language":"hcl","path":"website/docs/d/ami.html.markdown","slug":"ami","title":"ami"}}]}`))
			case category == "ephemeral-resources" && page == "1":
				w.Write([]byte(`{"data":[{"type":"provider-docs","id":"ephemeral-1","attributes":{"category":"ephemeral-resources","language":"hcl","path":"website/docs/ephemeral-resources/ecr_authorization_token.html.markdown","slug":"ecr_authorization_token","title":"ecr_authorization_token"}}]}`))
			case category == "actions" && page == "1":
				w.Write([]byte(`{"data":[{"type":"provider-docs","id":"action-1","attributes":{"category":"actions","language":"hcl","path":"website/docs/actions/cloudfront_create_invalidation.html.markdown","slug":"cloudfront_create_invalidation","title":"cloudfront_create_invalidation"}}]}`))
			case category == "functions" && page == "1":
				w.Write([]byte(`{"data":[{"type":"provider-docs","id":"function-1","attributes":{"category":"functions","language":"hcl","path":"website/docs/functions/arn_parse.html.markdown","slug":"arn_parse","title":"arn_parse"}}]}`))
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

	if len(items) != 107 {
		t.Fatalf("expected 107 supported docs, got %d", len(items))
	}
	if items[0].Kind != "overview" || items[0].Name != "provider" {
		t.Fatalf("unexpected first item: %#v", items[0])
	}
	if items[1].Kind != "guide" || items[1].Name != "custom-service-endpoints" {
		t.Fatalf("unexpected guide item: %#v", items[1])
	}
	if items[2].Kind != "resource" || items[2].Name != "aws_resource_000" {
		t.Fatalf("unexpected first resource item: %#v", items[2])
	}
	if items[102].Kind != "resource" || items[102].Name != "aws_tail_000" {
		t.Fatalf("expected second page resource, got %#v", items[102])
	}
	if items[103].Kind != "data" || items[103].Name != "aws_ami" {
		t.Fatalf("unexpected data item: %#v", items[103])
	}
	if items[104].Kind != "ephemeral" || items[104].Name != "aws_ecr_authorization_token" {
		t.Fatalf("unexpected ephemeral item: %#v", items[104])
	}
	if items[105].Kind != "action" || items[105].Name != "aws_cloudfront_create_invalidation" {
		t.Fatalf("unexpected action item: %#v", items[105])
	}
	if items[106].Kind != "function" || items[106].Name != "arn_parse" {
		t.Fatalf("unexpected function item: %#v", items[106])
	}
}

func TestStreamProviderDocsYieldsPages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/providers/hashicorp/aws":
			w.Write([]byte(`{
				"included": [
					{"type":"provider-versions","id":"97303","attributes":{"version":"6.46.0","tag":"v6.46.0"}}
				]
			}`))
		case "/v2/provider-docs":
			category := r.URL.Query().Get("filter[category]")
			page := r.URL.Query().Get("page[number]")
			switch {
			case category == "resources" && page == "1":
				w.Write([]byte(providerDocsResponseJSONWithTotalPages("resources", 1, "resource", 2)))
			case category == "resources" && page == "2":
				w.Write([]byte(providerDocsResponseJSONWithTotalPages("resources", 1, "tail", 2)))
			case category == "resources":
				t.Fatalf("unexpected resources page number: %s", page)
			default:
				w.Write([]byte(`{"data":[]}`))
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	var pageLengths []int
	err := client.StreamProviderDocs(context.Background(), Provider{
		Namespace:     "hashicorp",
		Name:          "aws",
		LatestVersion: "6.46.0",
	}, func(items []DocItem) error {
		pageLengths = append(pageLengths, len(items))
		return nil
	})
	if err != nil {
		t.Fatalf("stream provider docs: %v", err)
	}

	if fmt.Sprint(pageLengths) != "[1 1]" {
		t.Fatalf("unexpected page lengths: %v", pageLengths)
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

func TestGetProviderDocFetchesNewDocKinds(t *testing.T) {
	tests := []struct {
		name           string
		kind           string
		docName        string
		wantCategory   string
		wantSlugFilter string
		responseSlug   string
		responseTitle  string
		wantName       string
	}{
		{
			name:           "guide",
			kind:           "guide",
			docName:        "custom-service-endpoints",
			wantCategory:   "guides",
			wantSlugFilter: "custom-service-endpoints",
			responseSlug:   "custom-service-endpoints",
			responseTitle:  "Terraform AWS Provider Custom Service Endpoint Configuration",
			wantName:       "custom-service-endpoints",
		},
		{
			name:           "overview",
			kind:           "overview",
			docName:        "provider",
			wantCategory:   "overview",
			wantSlugFilter: "provider,index",
			responseSlug:   "index",
			responseTitle:  "overview",
			wantName:       "provider",
		},
		{
			name:           "ephemeral normalized",
			kind:           "ephemeral",
			docName:        "aws_ecr_authorization_token",
			wantCategory:   "ephemeral-resources",
			wantSlugFilter: "aws_ecr_authorization_token,ecr_authorization_token",
			responseSlug:   "ecr_authorization_token",
			responseTitle:  "ecr_authorization_token",
			wantName:       "aws_ecr_authorization_token",
		},
		{
			name:           "action raw",
			kind:           "action",
			docName:        "cloudfront_create_invalidation",
			wantCategory:   "actions",
			wantSlugFilter: "cloudfront_create_invalidation,aws_cloudfront_create_invalidation",
			responseSlug:   "cloudfront_create_invalidation",
			responseTitle:  "cloudfront_create_invalidation",
			wantName:       "aws_cloudfront_create_invalidation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/providers/hashicorp/aws":
					w.Write([]byte(`{
						"included": [
							{"type":"provider-versions","id":"97303","attributes":{"version":"6.46.0","tag":"v6.46.0"}}
						]
					}`))
				case "/v2/provider-docs":
					if got := r.URL.Query().Get("filter[category]"); got != tt.wantCategory {
						t.Fatalf("unexpected category: %q", got)
					}
					if got := r.URL.Query().Get("filter[slug]"); got != tt.wantSlugFilter {
						t.Fatalf("unexpected slug candidates: %q", got)
					}
					fmt.Fprintf(w, `{
						"data": [
							{"type":"provider-docs","id":"123","attributes":{"category":%q,"language":"hcl","path":"website/docs/example.html.markdown","slug":%q,"title":%q}}
						]
					}`, tt.wantCategory, tt.responseSlug, tt.responseTitle)
				case "/v2/provider-docs/123":
					fmt.Fprintf(w, `{
						"data": {"type":"provider-docs","id":"123","attributes":{"category":%q,"content":"# Doc\n","language":"hcl","path":"website/docs/example.html.markdown","slug":%q,"title":%q}}
					}`, tt.wantCategory, tt.responseSlug, tt.responseTitle)
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
			}, tt.kind, tt.docName)
			if err != nil {
				t.Fatalf("get provider doc: %v", err)
			}
			if page.Kind != tt.kind || page.Name != tt.wantName || page.Content != "# Doc" {
				t.Fatalf("unexpected page: %#v", page)
			}
		})
	}
}

func TestListProviderVersionsSortsNewestFirst(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/providers/hashicorp/aws" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("include"); got != "provider-versions" {
			t.Fatalf("unexpected include: %q", got)
		}
		w.Write([]byte(`{
			"included": [
				{"type":"provider-versions","id":"1","attributes":{"version":"6.44.0","published-at":"2026-05-06T18:00:00Z","tag":"v6.44.0"}},
				{"type":"provider-versions","id":"2","attributes":{"version":"6.46.0","published-at":"2026-05-20T18:00:00Z","tag":"v6.46.0"}},
				{"type":"provider-versions","id":"3","attributes":{"version":"6.45.0","published-at":"2026-05-13T18:00:00Z","tag":"v6.45.0"}}
			]
		}`))
	}))
	defer server.Close()

	client := NewClientForBaseURL(server.URL)
	versions, err := client.ListProviderVersions(context.Background(), Provider{Namespace: "hashicorp", Name: "aws"})
	if err != nil {
		t.Fatalf("list provider versions: %v", err)
	}

	got := []string{versions[0].Version, versions[1].Version, versions[2].Version}
	if fmt.Sprint(got) != "[6.46.0 6.45.0 6.44.0]" {
		t.Fatalf("unexpected versions: %#v", versions)
	}
	if versions[0].PublishedAt != "2026-05-20T18:00:00Z" {
		t.Fatalf("unexpected published date: %#v", versions[0])
	}
}

func providerDocsResponseJSON(category string, count int, prefix string) string {
	return providerDocsResponseJSONWithTotalPages(category, count, prefix, 0)
}

func providerDocsResponseJSONWithTotalPages(category string, count int, prefix string, totalPages int) string {
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
	b.WriteString(`]`)
	if totalPages > 0 {
		fmt.Fprintf(&b, `,"meta":{"pagination":{"total-pages":%d}}`, totalPages)
	}
	b.WriteString(`}`)
	return b.String()
}

func providerSearchResponseJSON(prefix string, count int) string {
	return providerSearchResponseJSONWithTotalPages(prefix, count, 0)
}

func providerSearchResponseJSONWithTotalPages(prefix string, count int, totalPages int) string {
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
	b.WriteString(`]`)
	if totalPages > 0 {
		fmt.Fprintf(&b, `,"meta":{"pagination":{"total-pages":%d}}`, totalPages)
	}
	b.WriteString(`}`)
	return b.String()
}
