package app

import (
	"context"

	"terraform-registry-cli/internal/project"
	"terraform-registry-cli/internal/registry"
)

type Provider struct {
	Source        string
	Namespace     string
	Name          string
	DisplayName   string
	LatestVersion string
	Downloads     int64
	Verified      bool
}

type ProjectResult struct {
	Provider     Provider
	ChangedFiles []string
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

type ProjectEditor interface {
	AddProvider(cwd string, providerInput string, opts AddProviderOptions) (ProjectResult, error)
	UpdateProvider(cwd string, providerInput string, opts UpdateProviderOptions) (ProjectResult, error)
	RemoveProvider(cwd string, providerInput string) (ProjectResult, error)
}

type Service struct {
	resolver ProviderResolver
	editor   ProjectEditor
}

func NewService(resolver ProviderResolver, editor ProjectEditor) Service {
	return Service{resolver: resolver, editor: editor}
}

func NewDefaultService() Service {
	return NewService(registryResolver{client: registry.NewClient()}, projectEditor{})
}

func (s Service) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
	return s.resolver.SearchProviders(ctx, query)
}

func (s Service) AddProvider(ctx context.Context, cwd string, providerInput string, versionConstraint string) (ProjectResult, error) {
	resolved, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return ProjectResult{}, err
	}

	result, err := s.editor.AddProvider(cwd, resolved.Namespace+"/"+resolved.Name, AddProviderOptions{VersionConstraint: versionConstraint})
	if err != nil {
		return ProjectResult{}, err
	}

	return result, nil
}

func (s Service) UpdateProvider(ctx context.Context, cwd string, providerInput string, versionConstraint string) (ProjectResult, error) {
	resolved, err := s.resolver.ResolveProvider(ctx, providerInput)
	if err != nil {
		return ProjectResult{}, err
	}

	result, err := s.editor.UpdateProvider(cwd, resolved.Namespace+"/"+resolved.Name, UpdateProviderOptions{VersionConstraint: versionConstraint})
	if err != nil {
		return ProjectResult{}, err
	}

	return result, nil
}

func (s Service) RemoveProvider(ctx context.Context, cwd string, providerInput string) (ProjectResult, error) {
	return s.editor.RemoveProvider(cwd, providerInput)
}

type registryResolver struct {
	client registry.Client
}

func (r registryResolver) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
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

func (r registryResolver) ResolveProvider(ctx context.Context, query string) (Provider, error) {
	provider, err := r.client.ResolveProvider(ctx, query)
	if err != nil {
		return Provider{}, err
	}
	return appProvider(provider), nil
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
		Namespace:     provider.Namespace,
		Name:          provider.Name,
		DisplayName:   provider.DisplayName,
		LatestVersion: provider.LatestVersion,
		Downloads:     provider.Downloads,
		Verified:      provider.Verified,
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
