package ovw

import "github.com/spf13/cobra"

const version = "0.1.0"

func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ovw",
		Short:   "Fast local project overview",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	return cmd
}
