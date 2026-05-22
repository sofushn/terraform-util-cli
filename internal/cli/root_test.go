package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"

	"terraform-util/internal/app"
)

var errFakeService = errors.New("service failed")

func execute(args ...string) (string, string, error) {
	return executeWithService(fakeService{
		providers: []app.Provider{{
			Namespace:     "hashicorp",
			Name:          "aws",
			DisplayName:   "aws",
			LatestVersion: "6.46.0",
			Verified:      true,
			Downloads:     500,
		}},
		projectResult: app.ProjectResult{
			Provider:          app.Provider{Source: "hashicorp/aws", Name: "aws"},
			VersionConstraint: "6.46.0",
			ChangedFiles:      []string{"providers.tf", "versions.tf"},
		},
		docItems: []app.DocItem{{
			Provider: app.Provider{Source: "registry.terraform.io/hashicorp/aws", LatestVersion: "6.46.0"},
			Kind:     "resource",
			Name:     "aws_vpc",
		}, {
			Provider: app.Provider{Source: "registry.terraform.io/hashicorp/aws", LatestVersion: "6.46.0"},
			Kind:     "data",
			Name:     "aws_ami",
		}},
		docPage: app.DocPage{
			Provider: app.Provider{Source: "registry.terraform.io/hashicorp/aws", LatestVersion: "6.46.0"},
			Kind:     "resource",
			Name:     "aws_vpc",
			Content:  "# Resource: aws_vpc",
			Source:   "https://github.com/hashicorp/terraform-provider-aws/blob/v6.46.0/website/docs/r/vpc.html.markdown",
			Website:  "https://registry.terraform.io/providers/hashicorp/aws/6.46.0/docs/resources/vpc",
		},
		versions: []app.ProviderVersion{{
			Provider:    app.Provider{Source: "registry.terraform.io/hashicorp/aws", LatestVersion: "6.46.0"},
			Version:     "6.46.0",
			PublishedAt: "2026-05-20T18:00:00Z",
		}, {
			Provider:    app.Provider{Source: "registry.terraform.io/hashicorp/aws", LatestVersion: "6.46.0"},
			Version:     "6.45.0",
			PublishedAt: "2026-05-13T18:00:00Z",
		}},
	}, args...)
}

func executeWithService(svc service, args ...string) (string, string, error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := newRootCommand(dependencies{service: svc})
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
		"terraform-util",
		"Registry Commands",
		"Terraform Project Commands",
		"search",
		"docs",
		"versions",
		"add",
		"--details",
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
		"--verbose",
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
			want: "# Resource: aws_vpc\n",
		},
		{
			name: "versions",
			args: []string{"versions", "aws"},
			want: "version\n6.46.0\n6.45.0\n",
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
	svc := fakeService{providers: []app.Provider{{
		Namespace:     "hashicorp",
		Name:          "aws",
		DisplayName:   "aws",
		LatestVersion: "6.46.0",
		Verified:      true,
		Downloads:     500,
		Tier:          "official",
	}, {
		Namespace:     "verylongnamespace",
		Name:          "custom",
		DisplayName:   "Custom",
		LatestVersion: "1.0.0",
		Downloads:     25,
	}}}

	stdout, _, err := executeWithService(svc, "search", "aws")
	if err != nil {
		t.Fatalf("expected search to succeed: %v", err)
	}

	want := "provider                          name      version                                     verified\n" +
		"hashicorp/aws                     aws       6.46.0                                      true    \n" +
		"verylongnamespace/custom          Custom    1.0.0                                               \n"
	if stdout != want {
		t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", want, stdout)
	}
}

func TestSearchCommandDetailsPrintsDownloads(t *testing.T) {
	svc := fakeService{providers: []app.Provider{{
		Namespace:     "hashicorp",
		Name:          "aws",
		DisplayName:   "aws",
		LatestVersion: "6.46.0",
		Verified:      true,
		Downloads:     500,
		Tier:          "official",
	}}}

	stdout, _, err := executeWithService(svc, "--details", "search", "aws")
	if err != nil {
		t.Fatalf("expected search to succeed: %v", err)
	}

	want := "provider                          name      version                                     downloads     tier        verified\n" +
		"hashicorp/aws                     aws       6.46.0                                      500           official    true    \n"
	if stdout != want {
		t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", want, stdout)
	}
}

func TestSearchCommandDetailsKeepsLongVersionsInVersionColumn(t *testing.T) {
	svc := fakeService{providers: []app.Provider{{
		Namespace:     "ArtsiomAntropau",
		Name:          "aws",
		DisplayName:   "aws",
		LatestVersion: "3.74.1+emr-managed-scaling-policy",
		Downloads:     4800,
		Tier:          "community",
	}}}

	stdout, _, err := executeWithService(svc, "--details", "search", "aws")
	if err != nil {
		t.Fatalf("expected search to succeed: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected header and one row, got:\n%s", stdout)
	}
	versionIndex := strings.Index(lines[1], "3.74.1+emr-managed-scaling-policy")
	downloadsIndex := strings.Index(lines[1], "4800")
	if versionIndex == -1 || downloadsIndex == -1 || downloadsIndex <= versionIndex+len("3.74.1+emr-managed-scaling-policy")+1 {
		t.Fatalf("expected long version to stay separated from downloads, got:\n%s", stdout)
	}
	if !strings.Contains(lines[0], "tier") || !strings.Contains(lines[1], "community") {
		t.Fatalf("expected details output to include tier, got:\n%s", stdout)
	}
}

func TestSearchCommandHandlesNoResults(t *testing.T) {
	stdout, _, err := executeWithService(fakeService{}, "search", "missing")
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
	svc := fakeService{projectResult: app.ProjectResult{
		Provider:          app.Provider{Source: "hashicorp/aws", Name: "aws"},
		VersionConstraint: "6.46.0",
		ChangedFiles:      []string{"providers.tf", "versions.tf"},
	}}

	stdout, _, err := executeWithService(svc, "add", "aws", "--version", "~> 6.0")
	if err != nil {
		t.Fatalf("expected add to succeed: %v", err)
	}
	for _, want := range []string{
		"Added provider hashicorp/aws (6.46.0)",
		"versions.tf",
		"providers.tf",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected add output to contain %q, got:\n%s", want, stdout)
		}
	}

	stdout, _, err = executeWithService(svc, "update", "aws", "--version", "~> 6.1")
	if err != nil {
		t.Fatalf("expected update to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Updated provider hashicorp/aws (6.46.0)") {
		t.Fatalf("unexpected update stdout:\n%s", stdout)
	}

	stdout, _, err = executeWithService(svc, "remove", "aws")
	if err != nil {
		t.Fatalf("expected remove to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Removed provider hashicorp/aws") {
		t.Fatalf("unexpected remove stdout:\n%s", stdout)
	}
}

func TestAddPrintsServiceResult(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	svc := fakeService{projectResult: app.ProjectResult{
		Provider:          app.Provider{Source: "popular/aws", Name: "aws"},
		VersionConstraint: "~> 1.0",
		ChangedFiles:      []string{"versions.tf"},
	}}

	stdout, _, err := executeWithService(svc, "add", "aws", "--version", "~> 1.0")
	if err != nil {
		t.Fatalf("expected add to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Added provider popular/aws") {
		t.Fatalf("expected resolved source in output, got:\n%s", stdout)
	}
}

func TestAddAndUpdateReturnServiceErrors(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	svc := fakeService{err: errFakeService}

	if _, _, err := executeWithService(svc, "add", "missing"); err == nil {
		t.Fatalf("expected add to fail when service fails")
	}
	if _, _, err := executeWithService(svc, "update", "missing", "--version", "~> 1.0"); err == nil {
		t.Fatalf("expected update to fail when service fails")
	}
}

func TestRemoveCallsService(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	svc := fakeService{projectResult: app.ProjectResult{
		Provider:     app.Provider{Source: "hashicorp/aws", Name: "aws"},
		ChangedFiles: []string{"versions.tf"},
	}}

	stdout, _, err := executeWithService(svc, "remove", "aws")
	if err != nil {
		t.Fatalf("expected remove to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Removed provider hashicorp/aws") {
		t.Fatalf("unexpected remove output:\n%s", stdout)
	}
}

func TestUpdateRequiresConstraint(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)
	svc := fakeService{projectResult: app.ProjectResult{
		Provider:          app.Provider{Source: "hashicorp/aws", Name: "aws"},
		VersionConstraint: "6.46.0",
		ChangedFiles:      []string{"versions.tf"},
	}}

	stdout, _, err := executeWithService(svc, "update", "aws")
	if err != nil {
		t.Fatalf("expected update without --version to use latest version: %v", err)
	}
	if !strings.Contains(stdout, "Updated provider hashicorp/aws (6.46.0)") {
		t.Fatalf("unexpected stdout:\n%s", stdout)
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
		"terraform-util search <provider>",
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
			want: "resource/aws_vpc\ndata/aws_ami\n",
		},
		{
			name: "with keyword",
			args: []string{"docs", "list", "aws", "vpc"},
			want: "resource/aws_vpc\n",
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

			want := "# Resource: aws_vpc\n"
			if stdout != want {
				t.Fatalf("unexpected stdout:\nwant: %q\n got: %q", want, stdout)
			}
		})
	}
}

func TestDocsVersionFlagSelectsProviderVersion(t *testing.T) {
	stdout, _, err := execute("--details", "docs", "--version", "5.0.0", "aws", "resource/aws_vpc")
	if err != nil {
		t.Fatalf("expected docs --version to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Version: 5.0.0") {
		t.Fatalf("expected docs output to use requested version, got:\n%s", stdout)
	}
}

func TestDocsVersionShorthandSelectsProviderVersion(t *testing.T) {
	stdout, _, err := execute("--details", "docs", "-v", "5.0.0", "aws", "resource/aws_vpc")
	if err != nil {
		t.Fatalf("expected docs -v to succeed: %v", err)
	}
	if !strings.Contains(stdout, "Version: 5.0.0") {
		t.Fatalf("expected docs output to use requested version, got:\n%s", stdout)
	}
}

func TestDocsDefaultsToLatestOption(t *testing.T) {
	recorder := &recordingService{fakeService: fakeService{
		docPage: app.DocPage{
			Provider: app.Provider{Source: "registry.terraform.io/hashicorp/aws", LatestVersion: "6.46.0"},
			Kind:     "resource",
			Name:     "aws_vpc",
			Content:  "# Resource: aws_vpc",
		},
	}}

	_, _, err := executeWithService(recorder, "docs", "aws", "resource/aws_vpc")
	if err != nil {
		t.Fatalf("expected docs to succeed: %v", err)
	}

	if !recorder.docsOptions.Latest || recorder.docsOptions.Version != "" {
		t.Fatalf("expected default docs options to mean latest, got %#v", recorder.docsOptions)
	}
}

func TestDocsListVersionFlagParses(t *testing.T) {
	_, _, err := execute("docs", "--version", "5.0.0", "list", "aws", "vpc")
	if err != nil {
		t.Fatalf("expected docs list --version to succeed: %v", err)
	}
}

func TestDocsListLatestFlagParses(t *testing.T) {
	_, _, err := execute("docs", "--latest", "list", "aws", "vpc")
	if err != nil {
		t.Fatalf("expected docs list --latest to succeed: %v", err)
	}
}

func TestDocsLatestConflictsWithVersion(t *testing.T) {
	_, _, err := execute("docs", "--latest", "--version", "5.0.0", "aws", "resource/aws_vpc")
	if err == nil {
		t.Fatalf("expected --latest and --version conflict to fail")
	}
}

func TestDocsListLatestConflictsWithVersion(t *testing.T) {
	_, _, err := execute("docs", "--latest", "--version", "5.0.0", "list", "aws")
	if err == nil {
		t.Fatalf("expected docs list --latest and --version conflict to fail")
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
	svc := fakeService{providers: []app.Provider{{
		Namespace:     "hashicorp",
		Name:          "aws",
		DisplayName:   "aws",
		LatestVersion: "6.46.0",
		Verified:      true,
		Downloads:     500,
	}}}

	stdout, _, err := executeWithService(svc,
		"--details",
		"search",
		"aws",
	)
	if err != nil {
		t.Fatalf("expected command with global flags to succeed: %v", err)
	}

	if stdout != "provider                          name      version                                     downloads     tier        verified\nhashicorp/aws                     aws       6.46.0                                      500                       true    \n" {
		t.Fatalf("unexpected stdout: %q", stdout)
	}
}

func TestDetailsShorthandParses(t *testing.T) {
	stdout, _, err := execute("-d", "search", "aws")
	if err != nil {
		t.Fatalf("expected -d to parse: %v", err)
	}
	if !strings.Contains(stdout, "downloads") {
		t.Fatalf("expected details output, got:\n%s", stdout)
	}
}

func TestVerboseFlagIsRejected(t *testing.T) {
	_, _, err := execute("--verbose", "search", "aws")
	if err == nil {
		t.Fatalf("expected --verbose to fail")
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
		{name: "update version", args: []string{"update", "aws", "--version", "~> 6.1"}},
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

func TestDocsDetailsOutputIncludesMetadata(t *testing.T) {
	stdout, _, err := execute("--details", "docs", "aws", "resource/aws_vpc")
	if err != nil {
		t.Fatalf("expected docs path to succeed: %v", err)
	}

	for _, want := range []string{
		"Provider: registry.terraform.io/hashicorp/aws",
		"Version: 6.46.0",
		"Website: https://registry.terraform.io/providers/hashicorp/aws/6.46.0/docs/resources/vpc",
		"Doc: resource/aws_vpc",
		"Source: https://github.com/hashicorp/terraform-provider-aws/blob/v6.46.0/website/docs/r/vpc.html.markdown",
		"# Resource: aws_vpc",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected details docs output to contain %q, got:\n%s", want, stdout)
		}
	}
}

func TestDocsListDetailsOutputIncludesProviderWebsite(t *testing.T) {
	stdout, _, err := execute("--details", "docs", "list", "aws", "vpc")
	if err != nil {
		t.Fatalf("expected docs list to succeed: %v", err)
	}

	for _, want := range []string{
		"Provider: registry.terraform.io/hashicorp/aws",
		"Version: 6.46.0",
		"Website: https://registry.terraform.io/providers/hashicorp/aws/6.46.0/docs",
		"resource/aws_vpc",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected details docs list output to contain %q, got:\n%s", want, stdout)
		}
	}
}

func TestVersionsDetailsOutputIncludesProviderWebsiteAndPublishedDates(t *testing.T) {
	stdout, _, err := execute("--details", "versions", "aws")
	if err != nil {
		t.Fatalf("expected versions details to succeed: %v", err)
	}

	for _, want := range []string{
		"provider: registry.terraform.io/hashicorp/aws",
		"website: https://registry.terraform.io/providers/hashicorp/aws",
		"version",
		"published",
		"6.46.0",
		"2026-05-20",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("expected versions details output to contain %q, got:\n%s", want, stdout)
		}
	}
}

type fakeService struct {
	providers     []app.Provider
	projectResult app.ProjectResult
	docItems      []app.DocItem
	docPage       app.DocPage
	versions      []app.ProviderVersion
	docsOptions   app.DocsOptions
	err           error
}

func (s fakeService) SearchProviders(ctx context.Context, query string) ([]app.Provider, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.providers, nil
}

func (s fakeService) StreamSearchProviders(ctx context.Context, query string, yield func([]app.Provider) error) error {
	if s.err != nil {
		return s.err
	}
	if len(s.providers) == 0 {
		return nil
	}
	return yield(s.providers)
}

func (s fakeService) ListProviderDocs(ctx context.Context, provider string, keyword string, opts app.DocsOptions) ([]app.DocItem, error) {
	if s.err != nil {
		return nil, s.err
	}
	if keyword == "" {
		return s.docItems, nil
	}

	filtered := make([]app.DocItem, 0, len(s.docItems))
	for _, item := range s.docItems {
		if strings.Contains(item.Kind+"/"+item.Name, keyword) {
			filtered = append(filtered, item)
		}
	}
	return filtered, nil
}

func (s fakeService) StreamProviderDocs(ctx context.Context, provider string, keyword string, opts app.DocsOptions, yield func([]app.DocItem) error) error {
	items, err := s.ListProviderDocs(ctx, provider, keyword, opts)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return nil
	}
	return yield(items)
}

func (s fakeService) GetProviderDoc(ctx context.Context, provider string, docsPath string, opts app.DocsOptions) (app.DocPage, error) {
	if s.err != nil {
		return app.DocPage{}, s.err
	}
	page := s.docPage
	if opts.Version != "" {
		page.Provider.LatestVersion = opts.Version
	}
	return page, nil
}

func (s fakeService) ListProviderVersions(ctx context.Context, provider string) ([]app.ProviderVersion, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.versions, nil
}

func (s fakeService) AddProvider(ctx context.Context, cwd string, provider string, versionConstraint string) (app.ProjectResult, error) {
	if s.err != nil {
		return app.ProjectResult{}, s.err
	}
	return s.projectResult, nil
}

func (s fakeService) UpdateProvider(ctx context.Context, cwd string, provider string, versionConstraint string) (app.ProjectResult, error) {
	if s.err != nil {
		return app.ProjectResult{}, s.err
	}
	return s.projectResult, nil
}

func (s fakeService) RemoveProvider(ctx context.Context, cwd string, provider string) (app.ProjectResult, error) {
	if s.err != nil {
		return app.ProjectResult{}, s.err
	}
	return s.projectResult, nil
}

type recordingService struct {
	fakeService
	mu          sync.Mutex
	docsOptions app.DocsOptions
}

func (s *recordingService) ListProviderDocs(ctx context.Context, provider string, keyword string, opts app.DocsOptions) ([]app.DocItem, error) {
	s.recordDocsOptions(opts)
	return s.fakeService.ListProviderDocs(ctx, provider, keyword, opts)
}

func (s *recordingService) StreamProviderDocs(ctx context.Context, provider string, keyword string, opts app.DocsOptions, yield func([]app.DocItem) error) error {
	s.recordDocsOptions(opts)
	return s.fakeService.StreamProviderDocs(ctx, provider, keyword, opts, yield)
}

func (s *recordingService) GetProviderDoc(ctx context.Context, provider string, docsPath string, opts app.DocsOptions) (app.DocPage, error) {
	s.recordDocsOptions(opts)
	return s.fakeService.GetProviderDoc(ctx, provider, docsPath, opts)
}

func (s *recordingService) recordDocsOptions(opts app.DocsOptions) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.docsOptions = opts
}
