package cli

import (
	"context"

	"github.com/sofushn/terraform-util-cli/internal/app"
)

type options struct {
	details bool
	quiet   bool
}

type service interface {
	SearchProviders(context.Context, string) ([]app.Provider, error)
	StreamSearchProviders(context.Context, string, func([]app.Provider) error) error
	StreamSearch(context.Context, string, app.SearchType, func([]app.SearchResult) error) error
	ListProviderDocs(context.Context, string, string, app.DocsOptions) ([]app.DocItem, error)
	StreamProviderDocs(context.Context, string, string, app.DocsOptions, func([]app.DocItem) error) error
	GetProviderDoc(context.Context, string, string, app.DocsOptions) (app.DocPage, error)
	GetModuleDoc(context.Context, string, app.DocsOptions) (app.ModuleDocPage, error)
	ListProviderVersions(context.Context, string) ([]app.ProviderVersion, error)
	ListModuleVersions(context.Context, string) ([]app.ModuleVersion, error)
	AddProvider(context.Context, string, string, string) (app.ProjectResult, error)
	UpdateProvider(context.Context, string, string, string) (app.ProjectResult, error)
	RemoveProvider(context.Context, string, string) (app.ProjectResult, error)
}

type dependencies struct {
	service service
}

type docsFlags struct {
	version string
	latest  bool
}
