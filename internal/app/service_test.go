package app

import (
	"context"
	"errors"
	"testing"
)

var errResolver = errors.New("resolver failed")

func TestAddProviderResolvesBeforeEditing(t *testing.T) {
	resolver := &fakeResolver{
		resolved: Provider{Namespace: "popular", Name: "aws", LatestVersion: "1.2.3"},
	}
	editor := &fakeEditor{}
	service := NewService(resolver, resolver, editor)

	result, err := service.AddProvider(context.Background(), "/work", "aws", "~> 1.0")
	if err != nil {
		t.Fatalf("add provider: %v", err)
	}

	if resolver.resolveCalls != 1 || resolver.resolveQuery != "aws" {
		t.Fatalf("expected resolver to be called with aws, got calls=%d query=%q", resolver.resolveCalls, resolver.resolveQuery)
	}
	if editor.addProvider != "popular/aws" {
		t.Fatalf("expected resolved provider to be passed to editor, got %q", editor.addProvider)
	}
	if editor.addVersion != "~> 1.0" {
		t.Fatalf("expected version constraint to be passed to editor, got %q", editor.addVersion)
	}
	if result.Provider.Source != "popular/aws" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.VersionConstraint != "~> 1.0" {
		t.Fatalf("expected result version constraint, got %q", result.VersionConstraint)
	}
}

func TestAddProviderUsesLatestVersionWhenOmitted(t *testing.T) {
	resolver := &fakeResolver{
		resolved: Provider{Namespace: "popular", Name: "aws", LatestVersion: "1.2.3"},
	}
	editor := &fakeEditor{}
	service := NewService(resolver, resolver, editor)

	if _, err := service.AddProvider(context.Background(), "/work", "aws", ""); err != nil {
		t.Fatalf("add provider: %v", err)
	}
	if editor.addVersion != "1.2.3" {
		t.Fatalf("expected latest version, got %q", editor.addVersion)
	}
}

func TestAddProviderReturnsResolverError(t *testing.T) {
	resolver := &fakeResolver{err: errResolver}
	editor := &fakeEditor{}
	service := NewService(resolver, resolver, editor)

	if _, err := service.AddProvider(context.Background(), "/work", "aws", ""); !errors.Is(err, errResolver) {
		t.Fatalf("expected resolver error, got %v", err)
	}
	if editor.addCalls != 0 {
		t.Fatalf("expected editor not to be called")
	}
}

func TestUpdateProviderResolvesBeforeEditing(t *testing.T) {
	resolver := &fakeResolver{
		resolved: Provider{Namespace: "hashicorp", Name: "aws", LatestVersion: "6.46.0"},
	}
	editor := &fakeEditor{}
	service := NewService(resolver, resolver, editor)

	if _, err := service.UpdateProvider(context.Background(), "/work", "aws", "~> 6.1"); err != nil {
		t.Fatalf("update provider: %v", err)
	}

	if resolver.resolveCalls != 1 {
		t.Fatalf("expected resolver to be called")
	}
	if editor.updateProvider != "hashicorp/aws" {
		t.Fatalf("expected resolved provider to be passed to editor, got %q", editor.updateProvider)
	}
	if editor.updateVersion != "~> 6.1" {
		t.Fatalf("expected constraint to be passed to editor, got %q", editor.updateVersion)
	}
}

func TestUpdateProviderUsesLatestVersionWhenOmitted(t *testing.T) {
	resolver := &fakeResolver{
		resolved: Provider{Namespace: "hashicorp", Name: "aws", LatestVersion: "6.46.0"},
	}
	editor := &fakeEditor{}
	service := NewService(resolver, resolver, editor)

	if _, err := service.UpdateProvider(context.Background(), "/work", "aws", ""); err != nil {
		t.Fatalf("update provider: %v", err)
	}
	if editor.updateVersion != "6.46.0" {
		t.Fatalf("expected latest version, got %q", editor.updateVersion)
	}
}

func TestRemoveProviderDoesNotResolve(t *testing.T) {
	resolver := &fakeResolver{err: errResolver}
	editor := &fakeEditor{}
	service := NewService(resolver, resolver, editor)

	if _, err := service.RemoveProvider(context.Background(), "/work", "aws"); err != nil {
		t.Fatalf("remove provider: %v", err)
	}

	if resolver.resolveCalls != 0 || resolver.searchCalls != 0 {
		t.Fatalf("expected resolver not to be called")
	}
	if editor.removeProvider != "aws" {
		t.Fatalf("expected local provider input to be passed to editor, got %q", editor.removeProvider)
	}
}

func TestSearchProvidersDelegatesToResolver(t *testing.T) {
	resolver := &fakeResolver{
		providers: []Provider{{
			Source:        "registry.terraform.io/hashicorp/aws",
			Namespace:     "hashicorp",
			Name:          "aws",
			DisplayName:   "aws",
			LatestVersion: "6.46.0",
			Downloads:     1,
			Verified:      true,
		}},
	}
	service := NewService(resolver, resolver, &fakeEditor{})

	providers, err := service.SearchProviders(context.Background(), "aws")
	if err != nil {
		t.Fatalf("search providers: %v", err)
	}
	if resolver.searchCalls != 1 || resolver.searchQuery != "aws" {
		t.Fatalf("expected resolver search call, got calls=%d query=%q", resolver.searchCalls, resolver.searchQuery)
	}
	if len(providers) != 1 || providers[0].Source != "registry.terraform.io/hashicorp/aws" {
		t.Fatalf("unexpected providers: %#v", providers)
	}
}

func TestStreamSearchProvidersDelegatesToResolver(t *testing.T) {
	resolver := &fakeResolver{
		providers: []Provider{{
			Source:        "registry.terraform.io/hashicorp/aws",
			Namespace:     "hashicorp",
			Name:          "aws",
			DisplayName:   "aws",
			LatestVersion: "6.46.0",
			Downloads:     1,
			Verified:      true,
		}},
	}
	service := NewService(resolver, resolver, &fakeEditor{})

	var pages [][]Provider
	err := service.StreamSearchProviders(context.Background(), "aws", func(providers []Provider) error {
		pages = append(pages, providers)
		return nil
	})
	if err != nil {
		t.Fatalf("stream search providers: %v", err)
	}
	if resolver.streamSearchCalls != 1 || resolver.searchQuery != "aws" {
		t.Fatalf("expected resolver stream search call, got calls=%d query=%q", resolver.streamSearchCalls, resolver.searchQuery)
	}
	if len(pages) != 1 || len(pages[0]) != 1 || pages[0][0].Source != "registry.terraform.io/hashicorp/aws" {
		t.Fatalf("unexpected pages: %#v", pages)
	}
}

func TestListProviderDocsResolvesAndFilters(t *testing.T) {
	resolver := &fakeResolver{
		resolved: Provider{
			Source:        "registry.terraform.io/hashicorp/aws",
			Namespace:     "hashicorp",
			Name:          "aws",
			LatestVersion: "6.46.0",
		},
		docs: []DocItem{{
			Kind: "resource",
			Name: "aws_vpc",
		}, {
			Kind: "data",
			Name: "aws_ami",
		}},
	}
	service := NewService(resolver, resolver, &fakeEditor{})

	items, err := service.ListProviderDocs(context.Background(), "aws", "vpc")
	if err != nil {
		t.Fatalf("list provider docs: %v", err)
	}

	if resolver.resolveCalls != 1 || resolver.listDocsCalls != 1 {
		t.Fatalf("expected resolve and docs calls, got resolve=%d docs=%d", resolver.resolveCalls, resolver.listDocsCalls)
	}
	if len(items) != 1 || items[0].Name != "aws_vpc" || items[0].Provider.Source != "registry.terraform.io/hashicorp/aws" {
		t.Fatalf("unexpected filtered docs: %#v", items)
	}
}

func TestStreamProviderDocsResolvesAndFilters(t *testing.T) {
	resolver := &fakeResolver{
		resolved: Provider{
			Source:        "registry.terraform.io/hashicorp/aws",
			Namespace:     "hashicorp",
			Name:          "aws",
			LatestVersion: "6.46.0",
		},
		docs: []DocItem{{
			Kind: "resource",
			Name: "aws_vpc",
		}, {
			Kind: "data",
			Name: "aws_ami",
		}},
	}
	service := NewService(resolver, resolver, &fakeEditor{})

	var pages [][]DocItem
	err := service.StreamProviderDocs(context.Background(), "aws", "vpc", func(items []DocItem) error {
		pages = append(pages, items)
		return nil
	})
	if err != nil {
		t.Fatalf("stream provider docs: %v", err)
	}
	if resolver.resolveCalls != 1 || resolver.streamDocsCalls != 1 {
		t.Fatalf("expected resolve and stream docs calls, got resolve=%d docs=%d", resolver.resolveCalls, resolver.streamDocsCalls)
	}
	if len(pages) != 1 || len(pages[0]) != 1 || pages[0][0].Name != "aws_vpc" || pages[0][0].Provider.Source != "registry.terraform.io/hashicorp/aws" {
		t.Fatalf("unexpected streamed docs: %#v", pages)
	}
}

func TestGetProviderDocResolvesAndFetchesPath(t *testing.T) {
	resolver := &fakeResolver{
		resolved: Provider{
			Source:        "registry.terraform.io/hashicorp/aws",
			Namespace:     "hashicorp",
			Name:          "aws",
			LatestVersion: "6.46.0",
		},
		docPage: DocPage{
			Kind:    "resource",
			Name:    "aws_vpc",
			Content: "# Resource: aws_vpc",
		},
	}
	service := NewService(resolver, resolver, &fakeEditor{})

	page, err := service.GetProviderDoc(context.Background(), "aws", "resource/aws_vpc")
	if err != nil {
		t.Fatalf("get provider doc: %v", err)
	}

	if resolver.docKind != "resource" || resolver.docName != "aws_vpc" {
		t.Fatalf("unexpected docs request kind=%q name=%q", resolver.docKind, resolver.docName)
	}
	if page.Provider.Source != "registry.terraform.io/hashicorp/aws" || page.Content != "# Resource: aws_vpc" {
		t.Fatalf("unexpected doc page: %#v", page)
	}
}

type fakeResolver struct {
	providers         []Provider
	resolved          Provider
	docs              []DocItem
	docPage           DocPage
	err               error
	searchCalls       int
	streamSearchCalls int
	searchQuery       string
	resolveCalls      int
	resolveQuery      string
	listDocsCalls     int
	streamDocsCalls   int
	docKind           string
	docName           string
}

func (r *fakeResolver) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
	r.searchCalls++
	r.searchQuery = query
	if r.err != nil {
		return nil, r.err
	}
	return r.providers, nil
}

func (r *fakeResolver) StreamSearchProviders(ctx context.Context, query string, yield func([]Provider) error) error {
	r.streamSearchCalls++
	r.searchQuery = query
	if r.err != nil {
		return r.err
	}
	return yield(r.providers)
}

func (r *fakeResolver) ResolveProvider(ctx context.Context, query string) (Provider, error) {
	r.resolveCalls++
	r.resolveQuery = query
	if r.err != nil {
		return Provider{}, r.err
	}
	return r.resolved, nil
}

func (r *fakeResolver) ListProviderDocs(ctx context.Context, provider Provider) ([]DocItem, error) {
	r.listDocsCalls++
	if r.err != nil {
		return nil, r.err
	}
	out := make([]DocItem, len(r.docs))
	copy(out, r.docs)
	for i := range out {
		out[i].Provider = provider
	}
	return out, nil
}

func (r *fakeResolver) StreamProviderDocs(ctx context.Context, provider Provider, yield func([]DocItem) error) error {
	r.streamDocsCalls++
	if r.err != nil {
		return r.err
	}
	out := make([]DocItem, len(r.docs))
	copy(out, r.docs)
	for i := range out {
		out[i].Provider = provider
	}
	return yield(out)
}

func (r *fakeResolver) GetProviderDoc(ctx context.Context, provider Provider, kind string, name string) (DocPage, error) {
	r.docKind = kind
	r.docName = name
	if r.err != nil {
		return DocPage{}, r.err
	}
	page := r.docPage
	page.Provider = provider
	return page, nil
}

type fakeEditor struct {
	addCalls       int
	addProvider    string
	addVersion     string
	updateProvider string
	updateVersion  string
	removeProvider string
}

func (e *fakeEditor) AddProvider(cwd string, providerInput string, opts AddProviderOptions) (ProjectResult, error) {
	e.addCalls++
	e.addProvider = providerInput
	e.addVersion = opts.VersionConstraint
	return ProjectResult{
		Provider:     Provider{Name: "aws", Source: providerInput},
		ChangedFiles: []string{"versions.tf"},
	}, nil
}

func (e *fakeEditor) UpdateProvider(cwd string, providerInput string, opts UpdateProviderOptions) (ProjectResult, error) {
	e.updateProvider = providerInput
	e.updateVersion = opts.VersionConstraint
	return ProjectResult{
		Provider:     Provider{Name: "aws", Source: providerInput},
		ChangedFiles: []string{"versions.tf"},
	}, nil
}

func (e *fakeEditor) RemoveProvider(cwd string, providerInput string) (ProjectResult, error) {
	e.removeProvider = providerInput
	return ProjectResult{
		Provider:     Provider{Name: providerInput, Source: "hashicorp/" + providerInput},
		ChangedFiles: []string{"versions.tf"},
	}, nil
}
