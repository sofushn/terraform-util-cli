package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"terraform-registry-cli/internal/project"
	"terraform-registry-cli/internal/registry"

	"github.com/spf13/cobra"
)

type options struct {
	verbose bool
	quiet   bool
}

type providerSearcher interface {
	SearchProviders(context.Context, string) ([]registry.Provider, error)
}

type dependencies struct {
	searcher providerSearcher
}

// NewRootCommand builds the terraform-registry command tree.
func NewRootCommand() *cobra.Command {
	return newRootCommand(dependencies{searcher: registry.NewClient()})
}

func newRootCommand(deps dependencies) *cobra.Command {
	opts := &options{}

	rootCmd := &cobra.Command{
		Use:           "terraform-registry",
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

	rootCmd.PersistentFlags().BoolVar(&opts.verbose, "verbose", false, "show additional output")
	rootCmd.PersistentFlags().BoolVar(&opts.quiet, "quiet", false, "suppress non-essential output")

	rootCmd.AddCommand(newSearchCommand(opts, deps.searcher))
	rootCmd.AddCommand(newAddCommand(opts))
	rootCmd.AddCommand(newRemoveCommand(opts))
	rootCmd.AddCommand(newUpdateCommand(opts))
	rootCmd.AddCommand(newDocsCommand(opts))

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

func newSearchCommand(opts *options, searcher providerSearcher) *cobra.Command {
	return &cobra.Command{
		Use:     "search <provider>",
		Short:   "Search providers",
		GroupID: "registry",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			providers, err := searcher.SearchProviders(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}
			if len(providers) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No providers found for %q\n", args[0])
				return nil
			}
			printProviderSearchResults(cmd.OutOrStdout(), providers, opts.verbose)
			return nil
		},
	}
}

func newAddCommand(opts *options) *cobra.Command {
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
			result, err := project.AddProvider(cwd, args[0], project.AddOptions{VersionConstraint: version})
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added provider %s", result.Provider.Source)
			if version != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", version)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			printChangedFiles(cmd.OutOrStdout(), result.ChangedFiles)
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "provider version constraint")

	return cmd
}

func newRemoveCommand(opts *options) *cobra.Command {
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
			result, err := project.RemoveProvider(cwd, args[0])
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

func newUpdateCommand(opts *options) *cobra.Command {
	var constraint string

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
			result, err := project.UpdateProvider(cwd, args[0], project.UpdateOptions{VersionConstraint: constraint})
			if err != nil {
				return err
			}
			if opts.quiet {
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated provider %s (%s)\n", result.Provider.Source, constraint)
			printChangedFiles(cmd.OutOrStdout(), result.ChangedFiles)
			return nil
		},
	}
	cmd.Flags().StringVar(&constraint, "constraint", "", "provider version constraint")

	return cmd
}

func newDocsCommand(opts *options) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "docs <provider> <data/name|resource/name|function/name>",
		Short:   "List or fetch provider docs",
		GroupID: "registry",
		Args:    validateDocsPathArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.quiet {
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "docs provider: %s path: %s\n", args[0], args[1])
			return nil
		},
	}

	cmd.AddCommand(newDocsListCommand(opts))

	return cmd
}

func newDocsListCommand(opts *options) *cobra.Command {
	return &cobra.Command{
		Use:   "list <provider> [keyword]",
		Short: "List provider docs",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.quiet {
				return nil
			}

			if len(args) == 2 {
				fmt.Fprintf(cmd.OutOrStdout(), "docs provider: %s list keyword: %s\n", args[0], args[1])
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "docs provider: %s list\n", args[0])
			return nil
		},
	}
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

func printProviderSearchResults(w io.Writer, providers []registry.Provider, verbose bool) {
	widths := searchColumnWidths(providers, verbose)
	printSearchRow(w, widths, verbose, []string{"provider", "name", "version", "downloads", "verified"})
	for _, provider := range providers {
		verified := ""
		if provider.Verified {
			verified = "verified"
		}

		downloads := ""
		if verbose {
			downloads = fmt.Sprintf("%d", provider.Downloads)
		}

		printSearchRow(w, widths, verbose, []string{
			provider.Namespace + "/" + provider.Name,
			provider.DisplayName,
			provider.LatestVersion,
			downloads,
			verified,
		})
	}
}

func searchColumnWidths(providers []registry.Provider, verbose bool) []int {
	widths := []int{len("provider"), len("name"), len("version"), len("downloads"), len("verified")}

	for _, provider := range providers {
		values := []string{
			provider.Namespace + "/" + provider.Name,
			provider.DisplayName,
			provider.LatestVersion,
			fmt.Sprintf("%d", provider.Downloads),
			"",
		}
		if provider.Verified {
			values[4] = "verified"
		}

		indexes := []int{0, 1, 2, 4}
		if verbose {
			indexes = []int{0, 1, 2, 3, 4}
		}
		for _, i := range indexes {
			if len(values[i]) > widths[i] {
				widths[i] = len(values[i])
			}
		}
	}

	return widths
}

func printSearchRow(w io.Writer, widths []int, verbose bool, values []string) {
	row := values
	rowWidths := widths
	if !verbose {
		row = []string{values[0], values[1], values[2], values[4]}
		rowWidths = []int{widths[0], widths[1], widths[2], widths[4]}
	}

	for i := 0; i < len(row); i++ {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprintf(w, "%-*s", rowWidths[i], row[i])
	}
	fmt.Fprintln(w)
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
