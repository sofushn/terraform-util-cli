package project

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclparse"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type Provider struct {
	LocalName string
	Source    string
}

type Result struct {
	Provider     Provider
	ChangedFiles []string
}

type AddOptions struct {
	VersionConstraint string
}

type UpdateOptions struct {
	VersionConstraint string
}

type Editor struct{}

func (Editor) AddProvider(cwd string, providerInput string, opts AddOptions) (Result, error) {
	return AddProvider(cwd, providerInput, opts)
}

func (Editor) UpdateProvider(cwd string, providerInput string, opts UpdateOptions) (Result, error) {
	return UpdateProvider(cwd, providerInput, opts)
}

func (Editor) RemoveProvider(cwd string, providerInput string) (Result, error) {
	return RemoveProvider(cwd, providerInput)
}

type tfFile struct {
	name    string
	path    string
	file    *hclwrite.File
	existed bool
	changed bool
	orig    []byte
}

func AddProvider(cwd string, providerInput string, opts AddOptions) (Result, error) {
	provider, err := ParseProvider(providerInput)
	if err != nil {
		return Result{}, err
	}

	files, err := loadFiles(cwd)
	if err != nil {
		return Result{}, err
	}

	reqFile := chooseRequiredProviderFile(cwd, &files)
	providerFile := chooseProviderBlockFile(cwd, &files)

	if err := upsertRequiredProvider(reqFile.file.Body(), provider, opts.VersionConstraint); err != nil {
		return Result{}, err
	}
	reqFile.changed = true

	if !hasProviderBlock(files, provider.LocalName) {
		block := hclwrite.NewBlock("provider", []string{provider.LocalName})
		providerFile.file.Body().AppendBlock(block)
		providerFile.file.Body().AppendNewline()
		providerFile.changed = true
	}

	changed, err := writeChanged(files)
	if err != nil {
		return Result{}, err
	}

	return Result{Provider: provider, ChangedFiles: changed}, nil
}

func RemoveProvider(cwd string, providerInput string) (Result, error) {
	provider, err := ParseProvider(providerInput)
	if err != nil {
		return Result{}, err
	}

	files, err := loadFiles(cwd)
	if err != nil {
		return Result{}, err
	}

	for _, file := range files {
		changed, source := removeRequiredProvider(file.file.Body(), provider.LocalName)
		if source != "" {
			provider.Source = source
		}
		if changed {
			file.changed = true
		}
		if removeEmptyProviderBlocks(file.file.Body(), provider.LocalName) {
			file.changed = true
		}
	}

	changed, err := writeChanged(files)
	if err != nil {
		return Result{}, err
	}

	return Result{Provider: provider, ChangedFiles: changed}, nil
}

func UpdateProvider(cwd string, providerInput string, opts UpdateOptions) (Result, error) {
	if strings.TrimSpace(opts.VersionConstraint) == "" {
		return Result{}, fmt.Errorf("update requires --constraint until registry version resolution is implemented")
	}

	provider, err := ParseProvider(providerInput)
	if err != nil {
		return Result{}, err
	}

	files, err := loadFiles(cwd)
	if err != nil {
		return Result{}, err
	}

	updated := false
	for _, file := range files {
		changed, err := updateRequiredProviderVersion(file.file.Body(), provider, opts.VersionConstraint)
		if err != nil {
			return Result{}, err
		}
		if changed {
			file.changed = true
			updated = true
		}
	}
	if !updated {
		return Result{}, fmt.Errorf("provider %q not found in required_providers; run add first", provider.LocalName)
	}

	changed, err := writeChanged(files)
	if err != nil {
		return Result{}, err
	}

	return Result{Provider: provider, ChangedFiles: changed}, nil
}

func ParseProvider(input string) (Provider, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return Provider{}, fmt.Errorf("provider is required")
	}

	parts := strings.Split(trimmed, "/")
	for _, part := range parts {
		if part == "" {
			return Provider{}, fmt.Errorf("invalid provider %q", input)
		}
	}

	switch len(parts) {
	case 1:
		return Provider{LocalName: parts[0], Source: "hashicorp/" + parts[0]}, nil
	case 2:
		return Provider{LocalName: parts[1], Source: trimmed}, nil
	case 3:
		return Provider{LocalName: parts[2], Source: trimmed}, nil
	default:
		return Provider{}, fmt.Errorf("invalid provider %q", input)
	}
}

func loadFiles(cwd string) ([]*tfFile, error) {
	entries, err := os.ReadDir(cwd)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".tf" {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	parser := hclparse.NewParser()
	files := make([]*tfFile, 0, len(names))
	for _, name := range names {
		path := filepath.Join(cwd, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}

		if _, diags := parser.ParseHCL(src, path); diags.HasErrors() {
			return nil, fmt.Errorf("invalid Terraform configuration in %s: %s", name, diags.Error())
		}

		file, diags := hclwrite.ParseConfig(src, path, hcl.Pos{Line: 1, Column: 1})
		if diags.HasErrors() {
			return nil, fmt.Errorf("invalid Terraform configuration in %s: %s", name, diags.Error())
		}

		files = append(files, &tfFile{name: name, path: path, file: file, existed: true, orig: src})
	}

	return files, nil
}

func chooseRequiredProviderFile(cwd string, files *[]*tfFile) *tfFile {
	for _, file := range *files {
		if hasBlock(file.file.Body(), "terraform") {
			return file
		}
	}
	return chooseOrCreateFile(cwd, files, "versions.tf")
}

func chooseProviderBlockFile(cwd string, files *[]*tfFile) *tfFile {
	for _, file := range *files {
		if hasBlock(file.file.Body(), "provider") {
			return file
		}
	}
	return chooseOrCreateFile(cwd, files, "providers.tf")
}

func chooseOrCreateFile(cwd string, files *[]*tfFile, name string) *tfFile {
	for _, file := range *files {
		if file.name == name {
			return file
		}
	}

	file := &tfFile{
		name: name,
		path: filepath.Join(cwd, name),
		file: hclwrite.NewEmptyFile(),
	}
	*files = append(*files, file)
	return file
}

func hasBlock(body *hclwrite.Body, blockType string) bool {
	for _, block := range body.Blocks() {
		if block.Type() == blockType {
			return true
		}
	}
	return false
}

func hasProviderBlock(files []*tfFile, localName string) bool {
	for _, file := range files {
		for _, block := range file.file.Body().Blocks() {
			if block.Type() == "provider" && len(block.Labels()) == 1 && block.Labels()[0] == localName {
				return true
			}
		}
	}
	return false
}

func upsertRequiredProvider(body *hclwrite.Body, provider Provider, versionConstraint string) error {
	requiredBody := requiredProvidersBody(body, true)
	attr := requiredBody.GetAttribute(provider.LocalName)
	if attr != nil {
		source, _ := providerSource(attr)
		if source != "" && source != provider.Source {
			return fmt.Errorf("provider %q already uses source %q", provider.LocalName, source)
		}
	}

	requiredBody.SetAttributeValue(provider.LocalName, providerValue(provider.Source, versionConstraint))
	return nil
}

func removeRequiredProvider(body *hclwrite.Body, localName string) (bool, string) {
	changed := false
	source := ""
	for _, block := range body.Blocks() {
		if block.Type() != "terraform" {
			continue
		}
		for _, nested := range block.Body().Blocks() {
			if nested.Type() == "required_providers" && nested.Body().GetAttribute(localName) != nil {
				if parsedSource, _ := providerSource(nested.Body().GetAttribute(localName)); parsedSource != "" {
					source = parsedSource
				}
				nested.Body().RemoveAttribute(localName)
				changed = true
			}
		}
	}
	return changed, source
}

func removeEmptyProviderBlocks(body *hclwrite.Body, localName string) bool {
	changed := false
	for _, block := range body.Blocks() {
		if block.Type() != "provider" || len(block.Labels()) != 1 || block.Labels()[0] != localName {
			continue
		}
		if len(block.Body().Attributes()) == 0 && len(block.Body().Blocks()) == 0 {
			if body.RemoveBlock(block) {
				changed = true
			}
		}
	}
	return changed
}

func updateRequiredProviderVersion(body *hclwrite.Body, provider Provider, constraint string) (bool, error) {
	updated := false
	for _, block := range body.Blocks() {
		if block.Type() != "terraform" {
			continue
		}
		for _, nested := range block.Body().Blocks() {
			if nested.Type() != "required_providers" {
				continue
			}
			attr := nested.Body().GetAttribute(provider.LocalName)
			if attr == nil {
				continue
			}

			source, _ := providerSource(attr)
			if source == "" {
				source = provider.Source
			}
			if source != provider.Source {
				return false, fmt.Errorf("provider %q already uses source %q", provider.LocalName, source)
			}
			nested.Body().SetAttributeValue(provider.LocalName, providerValue(source, constraint))
			updated = true
		}
	}
	return updated, nil
}

func requiredProvidersBody(body *hclwrite.Body, create bool) *hclwrite.Body {
	var terraformBlock *hclwrite.Block
	for _, block := range body.Blocks() {
		if block.Type() == "terraform" {
			terraformBlock = block
			break
		}
	}

	if terraformBlock == nil {
		if !create {
			return nil
		}
		terraformBlock = hclwrite.NewBlock("terraform", nil)
		body.AppendBlock(terraformBlock)
		body.AppendNewline()
	}

	for _, block := range terraformBlock.Body().Blocks() {
		if block.Type() == "required_providers" {
			return block.Body()
		}
	}

	if !create {
		return nil
	}
	requiredBlock := hclwrite.NewBlock("required_providers", nil)
	terraformBlock.Body().AppendBlock(requiredBlock)
	return requiredBlock.Body()
}

func providerValue(source string, versionConstraint string) cty.Value {
	values := map[string]cty.Value{
		"source": cty.StringVal(source),
	}
	if strings.TrimSpace(versionConstraint) != "" {
		values["version"] = cty.StringVal(versionConstraint)
	}
	return cty.ObjectVal(values)
}

func providerSource(attr *hclwrite.Attribute) (string, error) {
	expr, diags := hclsyntax.ParseExpression(attr.Expr().BuildTokens(nil).Bytes(), "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return "", fmt.Errorf(diags.Error())
	}

	value, diags := expr.Value(nil)
	if diags.HasErrors() || !value.Type().IsObjectType() {
		return "", nil
	}

	if !value.Type().HasAttribute("source") {
		return "", nil
	}

	source := value.GetAttr("source")
	if source.IsNull() || source.Type() != cty.String {
		return "", nil
	}

	return source.AsString(), nil
}

func writeChanged(files []*tfFile) ([]string, error) {
	var changed []string
	for _, file := range files {
		if !file.changed {
			continue
		}
		next := file.file.Bytes()
		if file.existed && string(file.orig) == string(next) {
			continue
		}
		if !file.existed && strings.TrimSpace(string(next)) == "" {
			continue
		}
		if err := writeFileAtomic(file.path, next); err != nil {
			return nil, err
		}
		changed = append(changed, file.name)
	}
	sort.Strings(changed)
	return changed, nil
}

func writeFileAtomic(path string, src []byte) error {
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()

	defer func() {
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(src); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	return os.Rename(tmpName, path)
}
