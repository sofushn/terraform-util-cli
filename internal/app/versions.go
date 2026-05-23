package app

import (
	"context"
	"fmt"
)

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

func (s Service) ListModuleVersions(ctx context.Context, moduleInput string) ([]ModuleVersion, error) {
	if s.modules == nil {
		return nil, fmt.Errorf("module registry is not configured")
	}
	return s.modules.ListModuleVersions(ctx, moduleInput)
}
