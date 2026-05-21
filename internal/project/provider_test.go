package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddProviderCreatesTerraformFiles(t *testing.T) {
	dir := t.TempDir()

	result, err := AddProvider(dir, "aws", AddOptions{VersionConstraint: "~> 6.0"})
	if err != nil {
		t.Fatalf("add provider: %v", err)
	}

	if result.Provider.LocalName != "aws" || result.Provider.Source != "hashicorp/aws" {
		t.Fatalf("unexpected provider: %#v", result.Provider)
	}
	assertChangedFiles(t, result.ChangedFiles, []string{"providers.tf", "versions.tf"})

	versions := readFile(t, dir, "versions.tf")
	for _, want := range []string{
		"terraform",
		"required_providers",
		"aws",
		`source  = "hashicorp/aws"`,
		`version = "~> 6.0"`,
	} {
		if !strings.Contains(versions, want) {
			t.Fatalf("expected versions.tf to contain %q, got:\n%s", want, versions)
		}
	}

	providers := readFile(t, dir, "providers.tf")
	if !strings.Contains(providers, `provider "aws"`) {
		t.Fatalf("expected provider block, got:\n%s", providers)
	}
}

func TestAddProviderIsIdempotent(t *testing.T) {
	dir := t.TempDir()

	if _, err := AddProvider(dir, "aws", AddOptions{VersionConstraint: "~> 6.0"}); err != nil {
		t.Fatalf("first add provider: %v", err)
	}
	beforeVersions := readFile(t, dir, "versions.tf")
	beforeProviders := readFile(t, dir, "providers.tf")

	result, err := AddProvider(dir, "aws", AddOptions{VersionConstraint: "~> 6.0"})
	if err != nil {
		t.Fatalf("second add provider: %v", err)
	}
	if len(result.ChangedFiles) != 0 {
		t.Fatalf("expected no changed files on second add, got %v", result.ChangedFiles)
	}

	if got := readFile(t, dir, "versions.tf"); got != beforeVersions {
		t.Fatalf("versions.tf changed on idempotent add:\nbefore:\n%s\nafter:\n%s", beforeVersions, got)
	}
	if got := readFile(t, dir, "providers.tf"); got != beforeProviders {
		t.Fatalf("providers.tf changed on idempotent add:\nbefore:\n%s\nafter:\n%s", beforeProviders, got)
	}
}

func TestAddProviderRejectsSourceConflict(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "versions.tf", `terraform {
  required_providers {
    aws = {
      source = "example/aws"
    }
  }
}
`)

	_, err := AddProvider(dir, "aws", AddOptions{VersionConstraint: "~> 6.0"})
	if err == nil {
		t.Fatalf("expected source conflict")
	}
	if !strings.Contains(err.Error(), `already uses source "example/aws"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateProviderUpdatesVersion(t *testing.T) {
	dir := t.TempDir()
	if _, err := AddProvider(dir, "hashicorp/aws", AddOptions{VersionConstraint: "~> 6.0"}); err != nil {
		t.Fatalf("add provider: %v", err)
	}

	result, err := UpdateProvider(dir, "aws", UpdateOptions{VersionConstraint: "~> 6.1"})
	if err != nil {
		t.Fatalf("update provider: %v", err)
	}
	assertChangedFiles(t, result.ChangedFiles, []string{"versions.tf"})

	versions := readFile(t, dir, "versions.tf")
	if !strings.Contains(versions, `version = "~> 6.1"`) {
		t.Fatalf("expected updated version, got:\n%s", versions)
	}
}

func TestUpdateProviderRequiresExistingProvider(t *testing.T) {
	dir := t.TempDir()

	_, err := UpdateProvider(dir, "aws", UpdateOptions{VersionConstraint: "~> 6.1"})
	if err == nil {
		t.Fatalf("expected missing provider error")
	}
	if !strings.Contains(err.Error(), "run add first") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveProviderPreservesConfiguredProviderBlock(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

provider "aws" {
  region = "us-east-1"
}
`)

	result, err := RemoveProvider(dir, "aws")
	if err != nil {
		t.Fatalf("remove provider: %v", err)
	}
	assertChangedFiles(t, result.ChangedFiles, []string{"main.tf"})

	main := readFile(t, dir, "main.tf")
	if strings.Contains(main, "hashicorp/aws") {
		t.Fatalf("expected required provider entry removed, got:\n%s", main)
	}
	if !strings.Contains(main, `provider "aws"`) || !strings.Contains(main, `region = "us-east-1"`) {
		t.Fatalf("expected configured provider block to remain, got:\n%s", main)
	}
}

func TestInvalidTerraformFailsBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "main.tf", `terraform {`)

	_, err := AddProvider(dir, "aws", AddOptions{VersionConstraint: "~> 6.0"})
	if err == nil {
		t.Fatalf("expected invalid Terraform error")
	}
	if _, statErr := os.Stat(filepath.Join(dir, "versions.tf")); !os.IsNotExist(statErr) {
		t.Fatalf("expected versions.tf not to be created, stat err: %v", statErr)
	}
}

func readFile(t *testing.T, dir string, name string) string {
	t.Helper()

	src, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(src)
}

func writeFile(t *testing.T, dir string, name string, content string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func assertChangedFiles(t *testing.T, got []string, want []string) {
	t.Helper()

	if len(got) != len(want) {
		t.Fatalf("changed files length mismatch: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("changed files mismatch: got %v, want %v", got, want)
		}
	}
}
