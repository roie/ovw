package ovw

import (
	"fmt"
	"os"

	"ovw/internal/app"
	"ovw/internal/cache"
	"ovw/internal/config"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

func NewRootCommand() *cobra.Command {
	opts := app.Options{}
	cmd := &cobra.Command{
		Use:     "ovw",
		Short:   "Fast local project overview",
		Version: version,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opts.Cwd = cwd
			opts.In = cmd.InOrStdin()
			opts.Out = cmd.OutOrStdout()
			return app.Run(opts)
		},
	}
	cmd.Flags().BoolVar(&opts.Plain, "plain", false, "force plain table output")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output JSON")
	cmd.Flags().StringVar(&opts.Status, "status", "", "filter by status")
	cmd.Flags().BoolVar(&opts.Dirty, "dirty", false, "show dirty projects")
	cmd.Flags().BoolVar(&opts.Stale, "stale", false, "show stale projects")
	cmd.Flags().BoolVar(&opts.Untagged, "untagged", false, "show untagged projects")
	cmd.Flags().StringVar(&opts.Sort, "sort", "", "sort by activity, name, or status")
	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newCacheCommand())
	return cmd
}

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ovw config",
	}
	configCmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print config path",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), paths.Config)
			return nil
		},
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Edit config",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			return config.Edit(paths.Config)
		},
	})
	return configCmd
}

func newCacheCommand() *cobra.Command {
	cacheCmd := &cobra.Command{
		Use:   "cache",
		Short: "Manage ovw cache",
	}
	cacheCmd.AddCommand(&cobra.Command{
		Use:   "clear",
		Short: "Clear generated cache",
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			if err := cache.Clear(paths.Cache); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Cache cleared.")
			return nil
		},
	})
	return cacheCmd
}
