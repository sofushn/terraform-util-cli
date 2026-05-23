package cli

import (
	"fmt"
	"strings"

	"github.com/sofushn/terraform-util-cli/internal/app"

	"github.com/spf13/cobra"
)

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
