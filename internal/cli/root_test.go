package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"terraform-registry-cli/internal/registry"
)

func execute(args ...string) (string, string, error) {
	return executeWithSearcher(registry.NewClient(), args...)
}

func executeWithSearcher(searcher providerSearcher, args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newRootCommand(dependencies{searcher: searcher})
	cmd.SetArgs(args)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)

	executedCmd, err := cmd.ExecuteC()
	if err != nil {
		helpCmd := executedCmd
		if helpCmd == nil {
			helpCmd = cmd
		}

		stderr.WriteString("Error: " + err.Error() + "\n\n")
		helpCmd.SetOut(&stderr)
		_ = helpCmd.Help()
	}

	return stdout.String(), stderr.String(), err
}

func chdir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get cwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func TestRootHelpWorks(t *testing.T) {
	stdout, _, err := execute("--help")
	if err != nil {
		t.Fatalf("expected root help to succeed: %v", err)
	}

	for _, want := range []string{
		"terraform-registry",
		"Registry Commands",
		"Terraform Project Commands",
		"search",
		"docs",
		"add",
		"--verbose",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected help output to contain %q, got:\n%s", want, stdout)
		}
	}

	for _, unwanted := range []string{
		"--timeout",
		"--cache-dir",
		"--no-cache",
		"--registry-url",
		"help        Help about any command",
	} {
		if strings.Contains(stdout, unwanted) {
			t.Fatalf("expected help output not to contain %q, got:\n%s", unwanted, stdout)
		}
	}
}

func TestDocumentedCommandsAcceptValidArguments(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "docs path",
			args: []string{"docs", "aws", "resource/aws_vpc"},
			want: "docs provider: aws path: resource/aws_vpc\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := execute(tt.args...)
			if err != nil {
				t.Fatalf("expected command to succeed: %v", err)
			}
			if stdout != tt.want {
				t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", tt.want, stdout)
			}
		})
	}
}

func TestSearchCommandPrintsRegistryResults(t *testing.T) {
	searcher := fakeSearcher{providers: []registry.Provider{{
		Namespace:     "hashicorp",
		Name:          "aws",
		DisplayName:   "aws",
		LatestVersion: "6.46.0",
		Verified:      true,
		Downloads:     500,
	}, {
		Namespace:     "verylongnamespace",
		Name:          "custom",
		DisplayName:   "Custom",
		LatestVersion: "1.0.0",
		Downloads:     25,
	}}}

	stdout, _, err := executeWithSearcher(searcher, "search", "aws")
	if err != nil {
		t.Fatalf("expected search to succeed: %v", err)
	}

	want := "provider                  name    version  verified\n" +
		"hashicorp/aws             aws     6.46.0   verified\n" +
		"verylongnamespace/custom  Custom  1.0.0            \n"
	if stdout != want {
		t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", want, stdout)
	}
}

func TestSearchCommandVerbosePrintsDownloads(t *testing.T) {
	searcher := fakeSearcher{providers: []registry.Provider{{
		Namespace:     "hashicorp",
		Name:          "aws",
		DisplayName:   "aws",
		LatestVersion: "6.46.0",
		Verified:      true,
		Downloads:     500,
	}}}

	stdout, _, err := executeWithSearcher(searcher, "--verbose", "search", "aws")
	if err != nil {
		t.Fatalf("expected search to succeed: %v", err)
	}

	want := "provider       name  version  downloads  verified\n" +
		"hashicorp/aws  aws   6.46.0   500        verified\n"
	if stdout != want {
		t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", want, stdout)
	}
}

func TestSearchCommandHandlesNoResults(t *testing.T) {
	stdout, _, err := executeWithSearcher(fakeSearcher{}, "search", "missing")
	if err != nil {
		t.Fatalf("expected search to succeed: %v", err)
	}
	if stdout != "No providers found for \"missing\"\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestProjectCommandsEditTerraformFiles(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	stdout, _, err := execute("add", "aws", "--version", "~> 6.0")
	if err != nil {
		t.Fatalf("expected add to succeed: %v", err)
	}
	for _, want := range []string{
		"Added provider hashicorp/aws (~> 6.0)",
		"versions.tf",
		"providers.tf",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected add output to contain %q, got:\n%s", want, stdout)
		}
	}

	versions, err := os.ReadFile(filepath.Join(dir, "versions.tf"))
	if err != nil {
		t.Fatalf("read versions.tf: %v", err)
	}
	for _, want := range []string{
		"required_providers",
		"aws",
		`source  = "hashicorp/aws"`,
		`version = "~> 6.0"`,
	} {
		if !strings.Contains(string(versions), want) {
			t.Fatalf("expected versions.tf to contain %q, got:\n%s", want, versions)
		}
	}

	providers, err := os.ReadFile(filepath.Join(dir, "providers.tf"))
	if err != nil {
		t.Fatalf("read providers.tf: %v", err)
	}
	if !strings.Contains(string(providers), `provider "aws"`) {
		t.Fatalf("expected providers.tf to contain provider block, got:\n%s", providers)
	}

	stdout, _, err = execute("update", "aws", "--constraint", "~> 6.1")
	if err != nil {
		t.Fatalf("expected update to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Updated provider hashicorp/aws (~> 6.1)") {
		t.Fatalf("unexpected update stdout:\n%s", stdout)
	}

	versions, err = os.ReadFile(filepath.Join(dir, "versions.tf"))
	if err != nil {
		t.Fatalf("read versions.tf: %v", err)
	}
	if !strings.Contains(string(versions), `version = "~> 6.1"`) {
		t.Fatalf("expected updated constraint, got:\n%s", versions)
	}

	stdout, _, err = execute("remove", "aws")
	if err != nil {
		t.Fatalf("expected remove to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Removed provider hashicorp/aws") {
		t.Fatalf("unexpected remove stdout:\n%s", stdout)
	}

	providers, err = os.ReadFile(filepath.Join(dir, "providers.tf"))
	if err != nil {
		t.Fatalf("read providers.tf: %v", err)
	}
	if strings.Contains(string(providers), `provider "aws"`) {
		t.Fatalf("expected provider block to be removed, got:\n%s", providers)
	}
}

func TestUpdateRequiresConstraint(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if _, _, err := execute("update", "aws"); err == nil {
		t.Fatalf("expected update without --constraint to fail")
	}
}

func TestMissingRequiredArgumentsFail(t *testing.T) {
	tests := [][]string{
		{"search"},
		{"add"},
		{"remove"},
		{"update"},
		{"docs"},
		{"docs", "aws"},
	}

	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			_, _, err := execute(args...)
			if err == nil {
				t.Fatalf("expected command to fail")
			}
		})
	}
}

func TestInvalidCommandShowsHelp(t *testing.T) {
	_, stderr, err := execute("bogus")
	if err == nil {
		t.Fatalf("expected invalid command to fail")
	}

	for _, want := range []string{
		"unknown command",
		"Usage:",
		"Registry Commands",
		"Terraform Project Commands",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestInvalidArgsShowCommandHelp(t *testing.T) {
	_, stderr, err := execute("search")
	if err == nil {
		t.Fatalf("expected missing args to fail")
	}

	for _, want := range []string{
		"accepts 1 arg(s), received 0",
		"Usage:",
		"terraform-registry search <provider>",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("expected stderr to contain %q, got:\n%s", want, stderr)
		}
	}
}

func TestDocsListWithAndWithoutKeyword(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "without keyword",
			args: []string{"docs", "list", "aws"},
			want: "docs provider: aws list\n",
		},
		{
			name: "with keyword",
			args: []string{"docs", "list", "aws", "vpc"},
			want: "docs provider: aws list keyword: vpc\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := execute(tt.args...)
			if err != nil {
				t.Fatalf("expected docs list to succeed: %v", err)
			}
			if stdout != tt.want {
				t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", tt.want, stdout)
			}
		})
	}
}

func TestDocsPathKindsParse(t *testing.T) {
	tests := []string{
		"resource/aws_vpc",
		"data/aws_ami",
		"function/templatestring",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			stdout, _, err := execute("docs", "aws", path)
			if err != nil {
				t.Fatalf("expected docs path to succeed: %v", err)
			}

			want := "docs provider: aws path: " + path + "\n"
			if stdout != want {
				t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", want, stdout)
			}
		})
	}
}

func TestInvalidDocsPathFails(t *testing.T) {
	_, _, err := execute("docs", "aws", "module/example")
	if err == nil {
		t.Fatalf("expected invalid docs path to fail")
	}
}

func TestDocsHelpShowsListSubcommand(t *testing.T) {
	stdout, _, err := execute("docs", "--help")
	if err != nil {
		t.Fatalf("expected docs help to succeed: %v", err)
	}

	for _, want := range []string{
		"docs <provider> <data/name|resource/name|function/name>",
		"list",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected docs help output to contain %q, got:\n%s", want, stdout)
		}
	}
}

func TestGlobalFlagsParse(t *testing.T) {
	searcher := fakeSearcher{providers: []registry.Provider{{
		Namespace:     "hashicorp",
		Name:          "aws",
		DisplayName:   "aws",
		LatestVersion: "6.46.0",
		Verified:      true,
		Downloads:     500,
	}}}

	stdout, _, err := executeWithSearcher(searcher,
		"--verbose",
		"search",
		"aws",
	)
	if err != nil {
		t.Fatalf("expected command with global flags to succeed: %v", err)
	}

	if stdout != "provider       name  version  downloads  verified\nhashicorp/aws  aws   6.46.0   500        verified\n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestQuietSuppressesPlaceholderOutput(t *testing.T) {
	stdout, _, err := execute("--quiet", "search", "aws")
	if err != nil {
		t.Fatalf("expected quiet command to succeed: %v", err)
	}
	if stdout != "" {
		t.Fatalf("expected quiet output to be empty, got %q", stdout)
	}
}

func TestCommandSpecificFutureFlagsParse(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	tests := []struct {
		name string
		args []string
	}{
		{name: "add version", args: []string{"add", "aws", "--version", "~> 6.0"}},
		{name: "update constraint", args: []string{"update", "aws", "--constraint", "~> 6.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := execute(tt.args...)
			if err != nil {
				t.Fatalf("expected command-specific flag to parse: %v", err)
			}
		})
	}
}

type fakeSearcher struct {
	providers []registry.Provider
	err       error
}

func (s fakeSearcher) SearchProviders(ctx context.Context, query string) ([]registry.Provider, error) {
	return s.providers, s.err
}
