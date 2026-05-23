package app

import "terraform-util/internal/registry"

type Service struct {
	resolver ProviderResolver
	docs     ProviderDocs
	modules  ModuleRegistry
	editor   ProjectEditor
}

func NewService(resolver ProviderResolver, docs ProviderDocs, editor ProjectEditor) Service {
	modules, _ := docs.(ModuleRegistry)
	return Service{resolver: resolver, docs: docs, modules: modules, editor: editor}
}

func NewDefaultService() Service {
	registryAdapter := registryAdapter{client: registry.NewClient()}
	return NewService(registryAdapter, registryAdapter, projectEditor{})
}
