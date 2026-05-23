package app

import "context"

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

type SearchType string

const (
	SearchTypeProvider SearchType = "provider"
	SearchTypeModule   SearchType = "module"
	SearchTypeAll      SearchType = "all"
)

type SearchResult struct {
	Type          SearchType
	Source        string
	RepositoryURL string
	Name          string
	LatestVersion string
	Downloads     int64
	Verified      bool
	Tier          string
}

type Module struct {
	Source        string
	RepositoryURL string
	Namespace     string
	Name          string
	Provider      string
	LatestVersion string
	Downloads     int64
	Verified      bool
	PublishedAt   string
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

type ModuleDocPage struct {
	Module  Module
	Content string
	Source  string
	Website string
}

type ProviderVersion struct {
	Provider    Provider
	Version     string
	PublishedAt string
}

type ModuleVersion struct {
	Module      Module
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

type ModuleRegistry interface {
	SearchModules(context.Context, string) ([]Module, error)
	StreamSearchModules(context.Context, string, func([]Module) error) error
	GetModuleDoc(context.Context, string, string) (ModuleDocPage, error)
	ListModuleVersions(context.Context, string) ([]ModuleVersion, error)
}

type ProjectEditor interface {
	AddProvider(cwd string, providerInput string, opts AddProviderOptions) (ProjectResult, error)
	UpdateProvider(cwd string, providerInput string, opts UpdateProviderOptions) (ProjectResult, error)
	RemoveProvider(cwd string, providerInput string) (ProjectResult, error)
}
