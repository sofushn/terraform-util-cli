package cli

import (
	"fmt"
	"os"
	"strings"

	"terraform-util/internal/app"

	"github.com/spf13/cobra"
)

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

func isDocsPath(path string) bool {
	for _, prefix := range []string{"data/", "resource/", "function/"} {
		if strings.HasPrefix(path, prefix) && len(path) > len(prefix) {
			return true
		}
	}
	return false
}
