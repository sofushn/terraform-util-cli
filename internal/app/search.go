package app

import (
	"context"
	"fmt"
	"strings"

	"github.com/sofushn/terraform-util-cli/internal/address"
)

func (s Service) SearchProviders(ctx context.Context, query string) ([]Provider, error) {
	return s.resolver.SearchProviders(ctx, query)
}

func (s Service) StreamSearchProviders(ctx context.Context, query string, yield func([]Provider) error) error {
	return s.resolver.StreamSearchProviders(ctx, query, yield)
}

func (s Service) StreamSearch(ctx context.Context, query string, searchType SearchType, yield func([]SearchResult) error) error {
	switch searchType {
	case SearchTypeProvider:
		return s.resolver.StreamSearchProviders(ctx, query, func(providers []Provider) error {
			return yield(providerSearchResults(providers))
		})
	case SearchTypeModule:
		if s.modules == nil {
			return fmt.Errorf("module registry is not configured")
		}
		return s.modules.StreamSearchModules(ctx, query, func(modules []Module) error {
			return yield(moduleSearchResults(modules))
		})
	case SearchTypeAll:
		if err := s.resolver.StreamSearchProviders(ctx, query, func(providers []Provider) error {
			return yield(providerSearchResults(providers))
		}); err != nil {
			return err
		}
		if s.modules == nil {
			return fmt.Errorf("module registry is not configured")
		}
		return s.modules.StreamSearchModules(ctx, query, func(modules []Module) error {
			return yield(moduleSearchResults(modules))
		})
	default:
		return fmt.Errorf("unknown search type %q", searchType)
	}
}

func providerSearchResults(providers []Provider) []SearchResult {
	out := make([]SearchResult, 0, len(providers))
	for _, provider := range providers {
		out = append(out, SearchResult{
			Type:          SearchTypeProvider,
			Source:        provider.Namespace + "/" + provider.Name,
			RepositoryURL: provider.RepositoryURL,
			Name:          provider.DisplayName,
			LatestVersion: provider.LatestVersion,
			Downloads:     provider.Downloads,
			Verified:      provider.Verified,
			Tier:          provider.Tier,
		})
	}
	return out
}

func moduleSearchResults(modules []Module) []SearchResult {
	out := make([]SearchResult, 0, len(modules))
	for _, module := range modules {
		out = append(out, SearchResult{
			Type:          SearchTypeModule,
			Source:        address.TrimRegistryHost(module.Source),
			RepositoryURL: module.RepositoryURL,
			Name:          module.Name,
			LatestVersion: module.LatestVersion,
			Downloads:     module.Downloads,
			Verified:      module.Verified,
		})
	}
	return out
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
