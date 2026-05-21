package app

import (
	"context"
	"fmt"
	"strings"

	"terraform-util/internal/project"
	"terraform-util/internal/registry"
)

type Provider struct {
	Source        string
	RepositoryURL string
	Namespace     string
	Name          string
	DisplayName   string
	LatestVersion string
	Downloads     int64
	Verified      bool
}

type ProjectResult struct {
	Provider          Provider
	VersionConstraint string
	ChangedFiles      []string
}

type DocItem struct {
	Provider Provider
	Kind     string
	Name     string
	Title    string
	Path     string
}

type DocPage struct {
	Provider Provider
	Kind     string
	Name     string
	Title    string
	Path     string
	Content  string
	Source   string
	Website  string
}

type AddProviderOptions struct {
	VersionConstraint string
}

type UpdateProviderOptions struct {
	VersionConstraint string
}

type ProviderResolver interface {
	SearchProviders(context.Context, string) ([]Provider, error)
	ResolveProvider(context.Context, string) (Provider, error)
}

type ProviderDocs interface {
	ListProviderDocs(context.Context, Provider) ([]DocItem, error)
	GetProviderDoc(context.Context, Provider, string, string) (DocPage, error)
}

type ProjectEditor interface {
	AddProvider(cwd string, providerInput string, opts AddProviderOptions) (ProjectResult, error)
	UpdateProvider(cwd string, providerInput string, opts UpdateProviderOptions) (ProjectResult, error)
	RemoveProvider(cwd string, providerInput string) (ProjectResult, error)
}

type Service struct {
	resolver ProviderResolver
	docs     ProviderDocs
	editor   ProjectEditor
}

func NewService(resolver ProviderResolver, docs ProviderDocs, editor ProjectEditor) Service {
	return Service{resolver: resolver, docs: docs, editor: editor}
}

func NewDefaultService() Service {
	registryAdapter := registryAdapter{client: registry.NewClient()}
	return NewService(registryAdapter, registryAdapter, projectEditor{})
}

func (s Service) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
	return s.resolver.SearchProviders(ctx, query)
}

func (s Service) ListProviderDocs(ctx context.Context, providerInput string, keyword string) ([]DocItem, error) {
	provider, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return nil, err
	}

	items, err := s.docs.ListProviderDocs(ctx, provider)
	if err != nil {
		return nil, err
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return items, nil
	}

	filtered := make([]DocItem, 0, len(items))
	for _, item := range items {
		if docItemMatches(item, keyword) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s Service) GetProviderDoc(ctx context.Context, providerInput string, docsPath string) (DocPage, error) {
	kind, name, err := parseDocsPath(docsPath)
	if err != nil {
		return DocPage{}, err
	}

	provider, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return DocPage{}, err
	}

	return s.docs.GetProviderDoc(ctx, provider, kind, name)
}

func (s Service) AddProvider(ctx context.Context, cwd string, providerInput string, versionConstraint string) (ProjectResult, error) {
	resolved, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return ProjectResult{}, err
	}

	versionConstraint = withDefaultVersion(versionConstraint, resolved.LatestVersion)
	result, err := s.editor.AddProvider(cwd, resolved.Namespace+"/"+resolved.Name, AddProviderOptions{VersionConstraint: versionConstraint})
	if err != nil {
		return ProjectResult{}, err
	}
	result.VersionConstraint = versionConstraint

	return result, nil
}

func (s Service) UpdateProvider(ctx context.Context, cwd string, providerInput string, versionConstraint string) (ProjectResult, error) {
	resolved, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return ProjectResult{}, err
	}

	versionConstraint = withDefaultVersion(versionConstraint, resolved.LatestVersion)
	result, err := s.editor.UpdateProvider(cwd, resolved.Namespace+"/"+resolved.Name, UpdateProviderOptions{VersionConstraint: versionConstraint})
	if err != nil {
		return ProjectResult{}, err
	}
	result.VersionConstraint = versionConstraint

	return result, nil
}

func (s Service) RemoveProvider(ctx context.Context, cwd string, providerInput string) (ProjectResult, error) {
	return s.editor.RemoveProvider(cwd, providerInput)
}

func withDefaultVersion(versionConstraint string, latestVersion string) string {
	if strings.TrimSpace(versionConstraint) != "" {
		return versionConstraint
	}
	return latestVersion
}

func parseDocsPath(path string) (string, string, error) {
	kind, name, ok := strings.Cut(path, "/")
	if !ok || strings.TrimSpace(name) == "" {
		return "", "", fmt.Errorf("docs path must start with data/, resource/, or function/")
	}
	switch kind {
	case "data", "resource", "function":
		return kind, name, nil
	default:
		return "", "", fmt.Errorf("docs path must start with data/, resource/, or function/")
	}
}

func docItemMatches(item DocItem, keyword string) bool {
	values := []string{
		item.Kind + "/" + item.Name,
		item.Name,
		item.Title,
		item.Path,
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(value), keyword) {
			return true
		}
	}
	return false
}

type registryAdapter struct {
	client registry.Client
}

func (r registryAdapter) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
	providers, err := r.client.SearchProviders(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]Provider, 0, len(providers))
	for _, provider := range providers {
		out = append(out, appProvider(provider))
	}
	return out, nil
}

func (r registryAdapter) ResolveProvider(ctx context.Context, query string) (Provider, error) {
	provider, err := r.client.ResolveProvider(ctx, query)
	if err != nil {
		return Provider{}, err
	}
	return appProvider(provider), nil
}

func (r registryAdapter) ListProviderDocs(ctx context.Context, provider Provider) ([]DocItem, error) {
	items, err := r.client.ListProviderDocs(ctx, registryProvider(provider))
	if err != nil {
		return nil, err
	}

	out := make([]DocItem, 0, len(items))
	for _, item := range items {
		out = append(out, appDocItem(provider, item))
	}
	return out, nil
}

func (r registryAdapter) GetProviderDoc(ctx context.Context, provider Provider, kind string, name string) (DocPage, error) {
	page, err := r.client.GetProviderDoc(ctx, registryProvider(provider), kind, name)
	if err != nil {
		return DocPage{}, err
	}
	return appDocPage(provider, page), nil
}

type projectEditor struct{}

func (projectEditor) AddProvider(cwd string, providerInput string, opts AddProviderOptions) (ProjectResult, error) {
	result, err := project.AddProvider(cwd, providerInput, project.AddOptions{VersionConstraint: opts.VersionConstraint})
	if err != nil {
		return ProjectResult{}, err
	}
	return appProjectResult(result), nil
}

func (projectEditor) UpdateProvider(cwd string, providerInput string, opts UpdateProviderOptions) (ProjectResult, error) {
	result, err := project.UpdateProvider(cwd, providerInput, project.UpdateOptions{VersionConstraint: opts.VersionConstraint})
	if err != nil {
		return ProjectResult{}, err
	}
	return appProjectResult(result), nil
}

func (projectEditor) RemoveProvider(cwd string, providerInput string) (ProjectResult, error) {
	result, err := project.RemoveProvider(cwd, providerInput)
	if err != nil {
		return ProjectResult{}, err
	}
	return appProjectResult(result), nil
}

func appProvider(provider registry.Provider) Provider {
	return Provider{
		Source:        provider.Source,
		RepositoryURL: provider.RepositoryURL,
		Namespace:     provider.Namespace,
		Name:          provider.Name,
		DisplayName:   provider.DisplayName,
		LatestVersion: provider.LatestVersion,
		Downloads:     provider.Downloads,
		Verified:      provider.Verified,
	}
}

func registryProvider(provider Provider) registry.Provider {
	return registry.Provider{
		Source:        provider.Source,
		RepositoryURL: provider.RepositoryURL,
		Namespace:     provider.Namespace,
		Name:          provider.Name,
		DisplayName:   provider.DisplayName,
		LatestVersion: provider.LatestVersion,
		Downloads:     provider.Downloads,
		Verified:      provider.Verified,
	}
}

func appDocItem(provider Provider, item registry.DocItem) DocItem {
	return DocItem{
		Provider: provider,
		Kind:     item.Kind,
		Name:     item.Name,
		Title:    item.Title,
		Path:     item.Path,
	}
}

func appDocPage(provider Provider, page registry.DocPage) DocPage {
	return DocPage{
		Provider: provider,
		Kind:     page.Kind,
		Name:     page.Name,
		Title:    page.Title,
		Path:     page.Path,
		Content:  page.Content,
		Source:   page.Source,
		Website:  page.Website,
	}
}

func appProjectResult(result project.Result) ProjectResult {
	return ProjectResult{
		Provider: Provider{
			Source: result.Provider.Source,
			Name:   result.Provider.LocalName,
		},
		ChangedFiles: result.ChangedFiles,
	}
}
