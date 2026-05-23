package cli

import "github.com/spf13/cobra"

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
