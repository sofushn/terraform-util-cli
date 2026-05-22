package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"terraform-util/internal/app"

	"github.com/spf13/cobra"
)

type options struct {
	details bool
	quiet   bool
}

type service interface {
	SearchProviders(context.Context, string) ([]app.Provider, error)
	StreamSearchProviders(context.Context, string, func([]app.Provider) error) error
	StreamSearch(context.Context, string, app.SearchType, func([]app.SearchResult) error) error
	ListProviderDocs(context.Context, string, string, app.DocsOptions) ([]app.DocItem, error)
	StreamProviderDocs(context.Context, string, string, app.DocsOptions, func([]app.DocItem) error) error
	GetProviderDoc(context.Context, string, string, app.DocsOptions) (app.DocPage, error)
	GetModuleDoc(context.Context, string, app.DocsOptions) (app.ModuleDocPage, error)
	ListProviderVersions(context.Context, string) ([]app.ProviderVersion, error)
	ListModuleVersions(context.Context, string) ([]app.ModuleVersion, error)
	AddProvider(context.Context, string, string, string) (app.ProjectResult, error)
	UpdateProvider(context.Context, string, string, string) (app.ProjectResult, error)
	RemoveProvider(context.Context, string, string) (app.ProjectResult, error)
}

type dependencies struct {
	service service
}

type docsFlags struct {
	version string
	latest  bool
}

// NewRootCommand builds the terraform-util command tree.
func NewRootCommand() *cobra.Command {
	return newRootCommand(dependencies{service: app.NewDefaultService()})
}

func newRootCommand(deps dependencies) *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:           "terraform-util",
		Short:         "Search Terraform Registry providers and docs",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	rootCmd.CompletionOptions.DisableDefaultCmd = true
	rootCmd.SetHelpCommand(&cobra.Command{Use: "help", Hidden: true})
	rootCmd.SetHelpTemplate(rootHelpTemplate)
	rootCmd.AddGroup(
		&cobra.Group{ID: "registry", Title: "Registry Commands"},
		&cobra.Group{ID: "project", Title: "Terraform Project Commands"},
	)

	rootCmd.PersistentFlags().BoolVarP(&opts.details, "details", "d", false, "show detailed output")
	rootCmd.PersistentFlags().BoolVar(&opts.quiet, "quiet", false, "suppress non-essential output")

	rootCmd.AddCommand(newSearchCommand(opts, deps.service))
	rootCmd.AddCommand(newVersionsCommand(opts, deps.service))
	rootCmd.AddCommand(newAddCommand(opts, deps.service))
	rootCmd.AddCommand(newRemoveCommand(opts, deps.service))
	rootCmd.AddCommand(newUpdateCommand(opts, deps.service))
	rootCmd.AddCommand(newDocsCommand(opts, deps.service))

	return rootCmd
}

// Execute runs the CLI and prints command help when parsing or validation fails.
func Execute(args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := NewRootCommand()
	cmd.SetArgs(args)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	executedCmd, err := cmd.ExecuteC()
	if err == nil {
		return nil
	}

	helpCmd := executedCmd
	if helpCmd == nil {
		helpCmd = cmd
	}

	fmt.Fprintf(stderr, "Error: %v\n\n", err)
	helpCmd.SetOut(stderr)
	_ = helpCmd.Help()

	return err
}

func newSearchCommand(opts *options, svc service) *cobra.Command {
	var typeFlag string
	var moduleOnly bool
	var providerOnly bool

	cmd := &cobra.Command{
		Use:     "search <query>",
		Short:   "Search providers and modules",
		GroupID: "registry",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			searchType, err := selectedSearchType(typeFlag, providerOnly, moduleOnly)
			if err != nil {
				return err
			}
			if strings.Join(strings.Fields(args[0]), " ") != args[0] || strings.Contains(args[0], " ") {
				return fmt.Errorf("search query must be a single token")
			}
			if opts.quiet {
				return nil
			}

			printed := false
			err = svc.StreamSearch(cmd.Context(), args[0], searchType, func(results []app.SearchResult) error {
				if !printed {
					printSearchHeader(cmd.OutOrStdout(), opts.details, searchType == app.SearchTypeAll)
					printed = true
				}
				printSearchRows(cmd.OutOrStdout(), results, opts.details, searchType == app.SearchTypeAll)
				return nil
			})
			if err != nil {
				return err
			}
			if !printed {
				fmt.Fprintf(cmd.OutOrStdout(), "No registry results found for %q\n", args[0])
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&typeFlag, "type", "t", string(app.SearchTypeProvider), "registry result type: provider, module, or all")
	cmd.Flags().BoolVarP(&providerOnly, "provider", "p", false, "search providers only")
	cmd.Flags().BoolVarP(&moduleOnly, "module", "m", false, "search modules only")
	return cmd
}

func newVersionsCommand(opts *options, svc service) *cobra.Command {
	return &cobra.Command{
		Use:     "versions <provider|module>",
		Short:   "List provider or module versions",
		GroupID: "registry",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if isModuleAddress(args[0]) {
				versions, err := svc.ListModuleVersions(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				if opts.quiet {
					return nil
				}
				printModuleVersions(cmd.OutOrStdout(), versions, opts.details)
				return nil
			}

			versions, err := svc.ListProviderVersions(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}
			printProviderVersions(cmd.OutOrStdout(), versions, opts.details)
			return nil
		},
	}
}

func newAddCommand(opts *options, svc service) *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:     "add <provider>",
		Short:   "Add a provider to the current Terraform project",
		GroupID: "project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			result, err := svc.AddProvider(cmd.Context(), cwd, args[0], version)
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added provider %s", result.Provider.Source)
			if result.VersionConstraint != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", result.VersionConstraint)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			printChangedFiles(cmd.OutOrStdout(), result.ChangedFiles)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "provider version constraint")

	return cmd
}

func newRemoveCommand(opts *options, svc service) *cobra.Command {
	return &cobra.Command{
		Use:     "remove <provider>",
		Short:   "Remove a provider from the current Terraform project",
		GroupID: "project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			result, err := svc.RemoveProvider(cmd.Context(), cwd, args[0])
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed provider %s\n", result.Provider.Source)
			printChangedFiles(cmd.OutOrStdout(), result.ChangedFiles)
			return nil
		},
	}
}

func newUpdateCommand(opts *options, svc service) *cobra.Command {
	var version string

	cmd := &cobra.Command{
		Use:     "update <provider>",
		Short:   "Update a provider version constraint",
		GroupID: "project",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			result, err := svc.UpdateProvider(cmd.Context(), cwd, args[0], version)
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated provider %s", result.Provider.Source)
			if result.VersionConstraint != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", result.VersionConstraint)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			printChangedFiles(cmd.OutOrStdout(), result.ChangedFiles)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "provider version constraint")

	return cmd
}

func newDocsCommand(opts *options, svc service) *cobra.Command {
	docsOpts := &docsFlags{}

	cmd := &cobra.Command{
		Use:     "docs <provider> <data/name|resource/name|function/name>|<module>",
		Short:   "List or fetch provider and module docs",
		GroupID: "registry",
		Args:    cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			appDocsOpts, err := appDocsOptions(*docsOpts)
			if err != nil {
				return err
			}

			if len(args) == 1 {
				if !isModuleAddress(args[0]) {
					return fmt.Errorf("provider docs require a docs path: data/name, resource/name, or function/name")
				}
				page, err := svc.GetModuleDoc(cmd.Context(), args[0], appDocsOpts)
				if err != nil {
					return err
				}
				if opts.quiet {
					return nil
				}
				printModuleDocPage(cmd.OutOrStdout(), page, opts.details)
				return nil
			}

			if isModuleAddress(args[0]) {
				return fmt.Errorf("module docs do not accept provider docs paths")
			}
			if !isDocsPath(args[1]) {
				return fmt.Errorf("docs path must start with data/, resource/, or function/")
			}

			appDocsOpts.CWD = currentWorkingDirectory()
			page, err := svc.GetProviderDoc(cmd.Context(), args[0], args[1], appDocsOpts)
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}

			printDocPage(cmd.OutOrStdout(), page, opts.details)
			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&docsOpts.version, "version", "v", "", "provider or module version for docs")
	cmd.PersistentFlags().BoolVar(&docsOpts.latest, "latest", false, "use latest provider or module version for docs")
	cmd.AddCommand(newDocsListCommand(opts, docsOpts, svc))

	return cmd
}

func newDocsListCommand(opts *options, docsOpts *docsFlags, svc service) *cobra.Command {
	return &cobra.Command{
		Use:   "list <provider> [keyword]",
		Short: "List provider docs",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			keyword := ""
			if len(args) == 2 {
				keyword = args[1]
			}
			if opts.quiet {
				return nil
			}

			appDocsOpts, err := appDocsOptions(*docsOpts)
			if err != nil {
				return err
			}
			appDocsOpts.CWD = currentWorkingDirectory()
			printedMetadata := false
			return svc.StreamProviderDocs(cmd.Context(), args[0], keyword, appDocsOpts, func(items []app.DocItem) error {
				if opts.details && !printedMetadata && len(items) > 0 {
					printProviderMetadata(cmd.OutOrStdout(), items[0].Provider, providerDocsWebsiteURL(items[0].Provider))
					fmt.Fprintln(cmd.OutOrStdout())
					printedMetadata = true
				}

				printDocList(cmd.OutOrStdout(), items, false)
				return nil
			})
		},
	}
}

func appDocsOptions(flags docsFlags) (app.DocsOptions, error) {
	version := strings.TrimSpace(flags.version)
	if version != "" && flags.latest {
		return app.DocsOptions{}, fmt.Errorf("--version and --latest cannot be used together")
	}
	return app.DocsOptions{Version: version, Latest: flags.latest}, nil
}

func currentWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

func validateDocsPathArgs(cmd *cobra.Command, args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("accepts 2 arg(s), received %d", len(args))
	}

	if !isDocsPath(args[1]) {
		return fmt.Errorf("docs path must start with data/, resource/, or function/")
	}

	return nil
}

func selectedSearchType(typeFlag string, providerOnly bool, moduleOnly bool) (app.SearchType, error) {
	if providerOnly && moduleOnly {
		return "", fmt.Errorf("--provider and --module cannot be used together")
	}
	normalized := app.SearchType(strings.ToLower(strings.TrimSpace(typeFlag)))
	switch normalized {
	case app.SearchTypeProvider, app.SearchTypeModule, app.SearchTypeAll:
	default:
		return "", fmt.Errorf("--type must be provider, module, or all")
	}
	if providerOnly && normalized != app.SearchTypeProvider {
		return "", fmt.Errorf("--provider cannot be used with --type %s", normalized)
	}
	if moduleOnly && normalized != app.SearchTypeProvider && normalized != app.SearchTypeModule {
		return "", fmt.Errorf("--module cannot be used with --type %s", normalized)
	}
	if providerOnly {
		return app.SearchTypeProvider, nil
	}
	if moduleOnly {
		return app.SearchTypeModule, nil
	}
	return normalized, nil
}

func isModuleAddress(input string) bool {
	source := strings.TrimPrefix(strings.TrimSpace(input), "registry.terraform.io/")
	parts := strings.Split(source, "/")
	return len(parts) == 3 && parts[0] != "" && parts[1] != "" && parts[2] != ""
}

func isDocsPath(path string) bool {
	for _, prefix := range []string{"data/", "resource/", "function/"} {
		if strings.HasPrefix(path, prefix) && len(path) > len(prefix) {
			return true
		}
	}
	return false
}

func printChangedFiles(w io.Writer, changedFiles []string) {
	if len(changedFiles) == 0 {
		fmt.Fprintln(w, "Changed: none")
		return
	}

	fmt.Fprintln(w, "Changed:")
	for _, name := range changedFiles {
		fmt.Fprintf(w, "  %s\n", name)
	}
}

func printDocList(w io.Writer, items []app.DocItem, details bool) {
	if details && len(items) > 0 {
		printProviderMetadata(w, items[0].Provider, providerDocsWebsiteURL(items[0].Provider))
		fmt.Fprintln(w)
	}

	for _, item := range items {
		fmt.Fprintf(w, "%s/%s\n", item.Kind, item.Name)
	}
}

func printDocPage(w io.Writer, page app.DocPage, details bool) {
	if details {
		printProviderMetadata(w, page.Provider, page.Website)
		fmt.Fprintf(w, "Doc: %s/%s\n", page.Kind, page.Name)
		if page.Source != "" {
			fmt.Fprintf(w, "Source: %s\n", page.Source)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, page.Content)
}

func printModuleDocPage(w io.Writer, page app.ModuleDocPage, details bool) {
	if details {
		fmt.Fprintln(w, "Type: module")
		printModuleMetadata(w, page.Module, page.Website)
		if page.Source != "" {
			fmt.Fprintf(w, "Source: %s\n", page.Source)
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintln(w, page.Content)
}

func printProviderVersions(w io.Writer, versions []app.ProviderVersion, details bool) {
	if details && len(versions) > 0 {
		provider := versions[0].Provider
		fmt.Fprintf(w, "provider: %s\n", provider.Source)
		fmt.Fprintf(w, "website: %s\n\n", providerWebsiteURLWithoutVersion(provider))
		printVersionsRow(w, []int{len("version"), len("published")}, []string{"version", "published"})
		for _, version := range versions {
			printVersionsRow(w, []int{len("version"), len("published")}, []string{version.Version, publishedDate(version.PublishedAt)})
		}
		return
	}

	fmt.Fprintln(w, "version")
	for _, version := range versions {
		fmt.Fprintln(w, version.Version)
	}
}

func printModuleVersions(w io.Writer, versions []app.ModuleVersion, details bool) {
	if details && len(versions) > 0 {
		module := versions[0].Module
		fmt.Fprintln(w, "type: module")
		fmt.Fprintf(w, "module: %s\n", module.Source)
		fmt.Fprintf(w, "website: %s\n", moduleWebsiteURLWithoutVersion(module))
		if module.RepositoryURL != "" {
			fmt.Fprintf(w, "source: %s\n", module.RepositoryURL)
		}
		fmt.Fprintln(w)
		printVersionsRow(w, []int{len("version"), len("published")}, []string{"version", "published"})
		for _, version := range versions {
			printVersionsRow(w, []int{len("version"), len("published")}, []string{version.Version, publishedDate(version.PublishedAt)})
		}
		return
	}

	fmt.Fprintln(w, "version")
	for _, version := range versions {
		fmt.Fprintln(w, version.Version)
	}
}

func printVersionsRow(w io.Writer, widths []int, values []string) {
	for i := 0; i < len(values); i++ {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", widths[i], values[i])
	}
	fmt.Fprintln(w)
}

func publishedDate(publishedAt string) string {
	if len(publishedAt) >= len("2006-01-02") {
		return publishedAt[:len("2006-01-02")]
	}
	return publishedAt
}

func printProviderMetadata(w io.Writer, provider app.Provider, website string) {
	fmt.Fprintf(w, "Provider: %s\n", provider.Source)
	fmt.Fprintf(w, "Version: %s\n", provider.LatestVersion)
	if website == "" {
		website = providerWebsiteURL(provider)
	}
	if website != "" {
		fmt.Fprintf(w, "Website: %s\n", website)
	}
}

func printModuleMetadata(w io.Writer, module app.Module, website string) {
	fmt.Fprintf(w, "Module: %s\n", module.Source)
	fmt.Fprintf(w, "Version: %s\n", module.LatestVersion)
	if website == "" {
		website = moduleWebsiteURL(module)
	}
	if website != "" {
		fmt.Fprintf(w, "Website: %s\n", website)
	}
}

func providerWebsiteURL(provider app.Provider) string {
	source := strings.TrimPrefix(provider.Source, "registry.terraform.io/")
	parts := strings.Split(source, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}

	version := provider.LatestVersion
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}

	return fmt.Sprintf("https://registry.terraform.io/providers/%s/%s/%s", parts[0], parts[1], version)
}

func providerWebsiteURLWithoutVersion(provider app.Provider) string {
	source := strings.TrimPrefix(provider.Source, "registry.terraform.io/")
	parts := strings.Split(source, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return ""
	}

	return fmt.Sprintf("https://registry.terraform.io/providers/%s/%s", parts[0], parts[1])
}

func providerDocsWebsiteURL(provider app.Provider) string {
	website := providerWebsiteURL(provider)
	if website == "" {
		return ""
	}
	return website + "/docs"
}

func moduleWebsiteURL(module app.Module) string {
	source := strings.TrimPrefix(module.Source, "registry.terraform.io/")
	parts := strings.Split(source, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}

	version := module.LatestVersion
	if strings.TrimSpace(version) == "" {
		version = "latest"
	}

	return fmt.Sprintf("https://registry.terraform.io/modules/%s/%s/%s/%s", parts[0], parts[1], parts[2], version)
}

func moduleWebsiteURLWithoutVersion(module app.Module) string {
	source := strings.TrimPrefix(module.Source, "registry.terraform.io/")
	parts := strings.Split(source, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}

	return fmt.Sprintf("https://registry.terraform.io/modules/%s/%s/%s", parts[0], parts[1], parts[2])
}

func printProviderSearchResults(w io.Writer, providers []app.Provider, details bool) {
	printProviderSearchHeader(w, details)
	printProviderSearchRows(w, providers, details)
}

func printSearchHeader(w io.Writer, details bool, includeType bool) {
	values := []string{"source", "name", "version", "downloads", "tier", "verified"}
	if includeType {
		values = append([]string{"type"}, values...)
	}
	printGenericSearchRow(w, searchColumnWidths(includeType), details, includeType, values)
}

func printSearchRows(w io.Writer, results []app.SearchResult, details bool, includeType bool) {
	widths := searchColumnWidths(includeType)
	for _, result := range results {
		verified := ""
		if result.Verified {
			verified = "true"
		}

		downloads := ""
		if details {
			downloads = fmt.Sprintf("%d", result.Downloads)
		}

		values := []string{
			result.Source,
			result.Name,
			result.LatestVersion,
			downloads,
			result.Tier,
			verified,
		}
		if includeType {
			values = append([]string{string(result.Type)}, values...)
		}
		printGenericSearchRow(w, widths, details, includeType, values)
	}
}

func printProviderSearchHeader(w io.Writer, details bool) {
	printSearchHeader(w, details, false)
}

func printProviderSearchRows(w io.Writer, providers []app.Provider, details bool) {
	printSearchRows(w, appProviderSearchResults(providers), details, false)
}

func appProviderSearchResults(providers []app.Provider) []app.SearchResult {
	results := make([]app.SearchResult, 0, len(providers))
	for _, provider := range providers {
		results = append(results, app.SearchResult{
			Type:          app.SearchTypeProvider,
			Source:        provider.Namespace + "/" + provider.Name,
			Name:          provider.DisplayName,
			LatestVersion: provider.LatestVersion,
			Downloads:     provider.Downloads,
			Verified:      provider.Verified,
			Tier:          provider.Tier,
		})
	}
	return results
}

func searchColumnWidths(includeType bool) []int {
	widths := []int{36, 10, 42, 12, 10, len("verified")}
	if includeType {
		return append([]int{len("provider")}, widths...)
	}
	return widths
}

func printGenericSearchRow(w io.Writer, widths []int, details bool, includeType bool, values []string) {
	row := values
	rowWidths := widths
	if !details {
		if includeType {
			row = []string{values[0], values[1], values[2], values[3], values[6]}
			rowWidths = []int{widths[0], widths[1], widths[2], widths[3], widths[6]}
		} else {
			row = []string{values[0], values[1], values[2], values[5]}
			rowWidths = []int{widths[0], widths[1], widths[2], widths[5]}
		}
	}

	for i := 0; i < len(row); i++ {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", rowWidths[i], truncateColumn(row[i], rowWidths[i]))
	}
	fmt.Fprintln(w)
}

func truncateColumn(value string, width int) string {
	if utf8.RuneCountInString(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:0]
	}

	runes := []rune(value)
	return string(runes[:width-1]) + "…"
}

const rootHelpTemplate = `{{with (or .Long .Short)}}{{.}}

{{end}}Usage:
{{if .Runnable}}  {{.UseLine}}
{{end}}{{if .HasAvailableSubCommands}}  {{.CommandPath}} [command]
{{end}}
{{if .HasAvailableSubCommands}}{{if .Groups}}{{range .Groups}}{{ $groupID := .ID }}{{.Title}}
{{range $.Commands}}{{if and (eq .GroupID $groupID) (not .Hidden)}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}
{{end}}{{end}}{{ $hasUngrouped := false }}{{range .Commands}}{{if and (not .Hidden) (not .GroupID)}}{{ $hasUngrouped = true }}{{end}}{{end}}{{if $hasUngrouped}}Available Commands:
{{range .Commands}}{{if and (not .Hidden) (not .GroupID)}}  {{rpad .Name .NamePadding }} {{.Short}}
{{end}}{{end}}
{{end}}{{end}}{{if .HasAvailableLocalFlags}}Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}{{if .HasAvailableInheritedFlags}}Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}

{{end}}{{if .HasAvailableSubCommands}}Use "{{.CommandPath}} [command] --help" for more information about a command.
{{end}}`
