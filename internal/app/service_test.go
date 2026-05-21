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
	service := NewService(resolver, editor)

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
	service := NewService(resolver, editor)

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
	service := NewService(resolver, editor)

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
	service := NewService(resolver, editor)

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
	service := NewService(resolver, editor)

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
	service := NewService(resolver, editor)

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
	service := NewService(resolver, &fakeEditor{})

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

type fakeResolver struct {
	providers    []Provider
	resolved     Provider
	err          error
	searchCalls  int
	searchQuery  string
	resolveCalls int
	resolveQuery string
}

func (r *fakeResolver) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
	r.searchCalls++
	r.searchQuery = query
	if r.err != nil {
		return nil, r.err
	}
	return r.providers, nil
}

func (r *fakeResolver) ResolveProvider(ctx context.Context, query string) (Provider, error) {
	r.resolveCalls++
	r.resolveQuery = query
	if r.err != nil {
		return Provider{}, r.err
	}
	return r.resolved, nil
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
