package app

import (
	"context"
	"strings"
)

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
