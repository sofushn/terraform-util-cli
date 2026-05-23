package cli

import (
	"fmt"
	"io"

	"github.com/sofushn/terraform-util-cli/internal/app"

	"github.com/spf13/cobra"
)

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
