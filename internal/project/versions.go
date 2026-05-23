package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"terraform-util/internal/address"

	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
)

type VersionHint struct {
	Version    string
	Source     string
	Constraint string
}

func FindProviderVersionHint(cwd string, providerInput string) (VersionHint, bool, error) {
	provider, err := ParseProvider(providerInput)
	if err != nil {
		return VersionHint{}, false, err
	}

	if hint, ok, err := findLockedProviderVersion(cwd, provider); ok || err != nil {
		return hint, ok, err
	}

	return findRequiredProviderVersion(cwd, provider)
}

func findLockedProviderVersion(cwd string, provider Provider) (VersionHint, bool, error) {
	path := filepath.Join(cwd, ".terraform.lock.hcl")
	src, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return VersionHint{}, false, nil
		}
		return VersionHint{}, false, err
	}

	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, path)
	if diags.HasErrors() {
		return VersionHint{}, false, fmt.Errorf("invalid Terraform lock file: %s", diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return VersionHint{}, false, nil
	}

	wantSource := canonicalProviderSource(provider.Source)
	for _, block := range body.Blocks {
		if block.Type != "provider" || len(block.Labels) != 1 {
			continue
		}
		source := canonicalProviderSource(block.Labels[0])
		if source != wantSource {
			continue
		}
		attr := block.Body.Attributes["version"]
		if attr == nil {
			return VersionHint{}, false, nil
		}
		version, ok := stringAttributeValue(attr)
		if !ok {
			return VersionHint{}, false, nil
		}
		return VersionHint{Version: version, Source: source}, true, nil
	}

	return VersionHint{}, false, nil
}

func findRequiredProviderVersion(cwd string, provider Provider) (VersionHint, bool, error) {
	files, err := loadFiles(cwd)
	if err != nil {
		return VersionHint{}, false, err
	}

	wantSource := canonicalProviderSource(provider.Source)
	for _, file := range files {
		hint, found, err := requiredProviderVersionInFile(file.path, file.orig, provider.LocalName, wantSource)
		if found || err != nil {
			return hint, found, err
		}
	}

	return VersionHint{}, false, nil
}

func requiredProviderVersionInFile(path string, src []byte, localName string, wantSource string) (VersionHint, bool, error) {
	parser := hclparse.NewParser()
	file, diags := parser.ParseHCL(src, path)
	if diags.HasErrors() {
		return VersionHint{}, false, fmt.Errorf("invalid Terraform configuration in %s: %s", filepath.Base(path), diags.Error())
	}

	body, ok := file.Body.(*hclsyntax.Body)
	if !ok {
		return VersionHint{}, false, nil
	}

	for _, terraformBlock := range body.Blocks {
		if terraformBlock.Type != "terraform" {
			continue
		}
		for _, requiredBlock := range terraformBlock.Body.Blocks {
			if requiredBlock.Type != "required_providers" {
				continue
			}
			attr := requiredBlock.Body.Attributes[localName]
			if attr == nil {
				continue
			}
			source, constraint := requiredProviderSourceAndVersion(attr, localName)
			if canonicalProviderSource(source) != wantSource {
				continue
			}
			if constraint == "" {
				return VersionHint{}, false, nil
			}
			return VersionHint{
				Version:    exactVersionConstraint(constraint),
				Source:     canonicalProviderSource(source),
				Constraint: constraint,
			}, true, nil
		}
	}

	return VersionHint{}, false, nil
}

func requiredProviderSourceAndVersion(attr *hclsyntax.Attribute, localName string) (string, string) {
	value, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return defaultProviderSource(localName), ""
	}

	if value.Type().IsObjectType() || value.Type().IsMapType() {
		source := defaultProviderSource(localName)
		version := ""
		for name, attrValue := range value.AsValueMap() {
			if !attrValue.IsKnown() || attrValue.Type() != cty.String {
				continue
			}
			switch name {
			case "source":
				source = attrValue.AsString()
			case "version":
				version = attrValue.AsString()
			}
		}
		return source, version
	}

	if value.Type() == cty.String {
		return value.AsString(), ""
	}

	return defaultProviderSource(localName), ""
}

func stringAttributeValue(attr *hclsyntax.Attribute) (string, bool) {
	value, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || value.Type() != cty.String {
		return "", false
	}
	return value.AsString(), true
}

func canonicalProviderSource(source string) string {
	return address.TrimRegistryHost(source)
}

func defaultProviderSource(localName string) string {
	return "hashicorp/" + localName
}

func exactVersionConstraint(constraint string) string {
	constraint = strings.TrimSpace(constraint)
	for _, marker := range []string{"~>", ">=", "<=", "!=", ">", "<", ","} {
		if strings.Contains(constraint, marker) {
			return ""
		}
	}
	return constraint
}
