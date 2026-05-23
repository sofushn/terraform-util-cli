package app

import (
	"context"

	"terraform-util/internal/project"
	"terraform-util/internal/registry"
)

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

func (r registryAdapter) SearchModules(ctx context.Context, query string) ([]Module, error) {
	modules, err := r.client.SearchModules(ctx, query)
	if err != nil {
		return nil, err
	}
	out := make([]Module, 0, len(modules))
	for _, module := range modules {
		out = append(out, appModule(module))
	}
	return out, nil
}

func (r registryAdapter) StreamSearchModules(ctx context.Context, query string, yield func([]Module) error) error {
	return r.client.StreamSearchModules(ctx, query, func(modules []registry.Module) error {
		out := make([]Module, 0, len(modules))
		for _, module := range modules {
			out = append(out, appModule(module))
		}
		return yield(out)
	})
}

func (r registryAdapter) GetModuleDoc(ctx context.Context, moduleInput string, version string) (ModuleDocPage, error) {
	page, err := r.client.GetModuleDoc(ctx, moduleInput, version)
	if err != nil {
		return ModuleDocPage{}, err
	}
	return ModuleDocPage{
		Module:  appModule(page.Module),
		Content: page.Content,
		Source:  page.Source,
		Website: page.Website,
	}, nil
}

func (r registryAdapter) ListModuleVersions(ctx context.Context, moduleInput string) ([]ModuleVersion, error) {
	versions, err := r.client.ListModuleVersions(ctx, moduleInput)
	if err != nil {
		return nil, err
	}
	out := make([]ModuleVersion, 0, len(versions))
	for _, version := range versions {
		out = append(out, ModuleVersion{
			Module:      appModule(version.Module),
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

func appModule(module registry.Module) Module {
	return Module{
		Source:        module.Source,
		RepositoryURL: module.RepositoryURL,
		Namespace:     module.Namespace,
		Name:          module.Name,
		Provider:      module.Provider,
		LatestVersion: module.LatestVersion,
		Downloads:     module.Downloads,
		Verified:      module.Verified,
		PublishedAt:   module.PublishedAt,
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
