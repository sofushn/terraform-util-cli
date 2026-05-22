package app

import (
	"context"
	"fmt"
	"strings"

	"terraform-util/internal/project"
	"terraform-util/internal/registry"

	goversion "github.com/hashicorp/go-version"
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
	Tier          string
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

type ProviderVersion struct {
	Provider    Provider
	Version     string
	PublishedAt string
}

type DocsOptions struct {
	Version string
	Latest  bool
	CWD     string
}

type AddProviderOptions struct {
	VersionConstraint string
}

type UpdateProviderOptions struct {
	VersionConstraint string
}

type ProviderResolver interface {
	SearchProviders(context.Context, string) ([]Provider, error)
	StreamSearchProviders(context.Context, string, func([]Provider) error) error
	ResolveProvider(context.Context, string) (Provider, error)
}

type ProviderDocs interface {
	ListProviderDocs(context.Context, Provider) ([]DocItem, error)
	StreamProviderDocs(context.Context, Provider, func([]DocItem) error) error
	GetProviderDoc(context.Context, Provider, string, string) (DocPage, error)
	ListProviderVersions(context.Context, Provider) ([]ProviderVersion, error)
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

func (s Service) StreamSearchProviders(ctx context.Context, query string, yield func([]Provider) error) error {
	return s.resolver.StreamSearchProviders(ctx, query, yield)
}

func (s Service) ListProviderDocs(ctx context.Context, providerInput string, keyword string, opts DocsOptions) ([]DocItem, error) {
	provider, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return nil, err
	}
	provider, err = s.providerForDocsVersion(ctx, provider, providerInput, opts)
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

func (s Service) StreamProviderDocs(ctx context.Context, providerInput string, keyword string, opts DocsOptions, yield func([]DocItem) error) error {
	provider, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return err
	}
	provider, err = s.providerForDocsVersion(ctx, provider, providerInput, opts)
	if err != nil {
		return err
	}

	keyword = strings.ToLower(strings.TrimSpace(keyword))
	return s.docs.StreamProviderDocs(ctx, provider, func(items []DocItem) error {
		out := make([]DocItem, 0, len(items))
		for _, item := range items {
			item.Provider = provider
			if keyword == "" || docItemMatches(item, keyword) {
				out = append(out, item)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return yield(out)
	})
}

func (s Service) GetProviderDoc(ctx context.Context, providerInput string, docsPath string, opts DocsOptions) (DocPage, error) {
	kind, name, err := parseDocsPath(docsPath)
	if err != nil {
		return DocPage{}, err
	}

	provider, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return DocPage{}, err
	}
	provider, err = s.providerForDocsVersion(ctx, provider, providerInput, opts)
	if err != nil {
		return DocPage{}, err
	}

	return s.docs.GetProviderDoc(ctx, provider, kind, name)
}

func (s Service) ListProviderVersions(ctx context.Context, providerInput string) ([]ProviderVersion, error) {
	provider, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return nil, err
	}

	versions, err := s.docs.ListProviderVersions(ctx, provider)
	if err != nil {
		return nil, err
	}

	for i := range versions {
		versions[i].Provider = provider
	}
	return versions, nil
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

func (s Service) providerForDocsVersion(ctx context.Context, provider Provider, providerInput string, opts DocsOptions) (Provider, error) {
	if opts.Latest {
		return provider, nil
	}
	if strings.TrimSpace(opts.Version) != "" {
		provider.LatestVersion = strings.TrimSpace(opts.Version)
		return provider, nil
	}

	if strings.TrimSpace(opts.CWD) == "" {
		return provider, nil
	}

	hint, ok, err := project.FindProviderVersionHint(opts.CWD, provider.Namespace+"/"+provider.Name)
	if err != nil {
		return Provider{}, err
	}
	if !ok && providerInput != provider.Namespace+"/"+provider.Name {
		hint, ok, err = project.FindProviderVersionHint(opts.CWD, providerInput)
		if err != nil {
			return Provider{}, err
		}
	}
	if !ok {
		return provider, nil
	}
	if strings.TrimSpace(hint.Version) != "" {
		provider.LatestVersion = strings.TrimSpace(hint.Version)
		return provider, nil
	}
	if strings.TrimSpace(hint.Constraint) == "" {
		return provider, nil
	}

	version, ok, err := s.newestMatchingProviderVersion(ctx, provider, hint.Constraint)
	if err != nil {
		return Provider{}, err
	}
	if ok {
		provider.LatestVersion = version
	}
	return provider, nil
}

func (s Service) newestMatchingProviderVersion(ctx context.Context, provider Provider, constraint string) (string, bool, error) {
	constraints, err := goversion.NewConstraint(constraint)
	if err != nil {
		return "", false, nil
	}

	versions, err := s.docs.ListProviderVersions(ctx, provider)
	if err != nil {
		return "", false, err
	}
	for _, candidate := range versions {
		version, err := goversion.NewVersion(candidate.Version)
		if err != nil {
			continue
		}
		if constraints.Check(version) {
			return candidate.Version, true, nil
		}
	}
	return "", false, nil
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

func (r registryAdapter) StreamSearchProviders(ctx context.Context, query string, yield func([]Provider) error) error {
	return r.client.StreamSearchProviders(ctx, query, func(providers []registry.Provider) error {
		out := make([]Provider, 0, len(providers))
		for _, provider := range providers {
			out = append(out, appProvider(provider))
		}
		return yield(out)
	})
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

func (r registryAdapter) StreamProviderDocs(ctx context.Context, provider Provider, yield func([]DocItem) error) error {
	return r.client.StreamProviderDocs(ctx, registryProvider(provider), func(items []registry.DocItem) error {
		out := make([]DocItem, 0, len(items))
		for _, item := range items {
			out = append(out, appDocItem(provider, item))
		}
		return yield(out)
	})
}

func (r registryAdapter) GetProviderDoc(ctx context.Context, provider Provider, kind string, name string) (DocPage, error) {
	page, err := r.client.GetProviderDoc(ctx, registryProvider(provider), kind, name)
	if err != nil {
		return DocPage{}, err
	}
	return appDocPage(provider, page), nil
}

func (r registryAdapter) ListProviderVersions(ctx context.Context, provider Provider) ([]ProviderVersion, error) {
	versions, err := r.client.ListProviderVersions(ctx, registryProvider(provider))
	if err != nil {
		return nil, err
	}

	out := make([]ProviderVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, ProviderVersion{
			Provider:    provider,
			Version:     version.Version,
			PublishedAt: version.PublishedAt,
		})
	}
	return out, nil
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
		Tier:          provider.Tier,
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
		Tier:          provider.Tier,
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
