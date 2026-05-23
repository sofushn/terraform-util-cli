package app

import (
	"context"
	"fmt"
	"strings"

	"terraform-util/internal/project"

	goversion "github.com/hashicorp/go-version"
)

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

func (s Service) GetModuleDoc(ctx context.Context, moduleInput string, opts DocsOptions) (ModuleDocPage, error) {
	if s.modules == nil {
		return ModuleDocPage{}, fmt.Errorf("module registry is not configured")
	}
	version := ""
	if strings.TrimSpace(opts.Version) != "" {
		version = strings.TrimSpace(opts.Version)
	}
	return s.modules.GetModuleDoc(ctx, moduleInput, version)
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
