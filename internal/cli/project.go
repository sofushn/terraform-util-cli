package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

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
