package ovw

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"ovw/internal/app"
	"ovw/internal/cache"
	"ovw/internal/config"
	"ovw/internal/metadata"
	"ovw/internal/scanner"

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
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newAddCommand())
	cmd.AddCommand(newVisibilityCommand("hide", true))
	cmd.AddCommand(newVisibilityCommand("remove", true))
	cmd.AddCommand(newVisibilityCommand("unhide", false))
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

func newScanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "scan",
		Short: "Scan configured roots",
		RunE: func(cmd *cobra.Command, args []string) error {
			start := time.Now()
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			cfg, _, err := config.Ensure(paths.Config, cwd, cmd.InOrStdin(), cmd.OutOrStdout())
			if err != nil {
				return err
			}
			meta, err := metadata.Load(paths.Metadata)
			if err != nil {
				return err
			}
			for _, root := range cfg.Roots {
				fmt.Fprintf(cmd.OutOrStdout(), "Scanning %s...\n", root)
			}
			projects, err := scanner.Scan(cfg, meta)
			if err != nil {
				return err
			}
			cacheStore := cache.New()
			for _, project := range projects {
				cacheStore.Projects[project.Path] = cache.Project{}
			}
			if err := cache.Write(paths.Cache, cacheStore); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Found %d projects in %.1fs\n", len(projects), time.Since(start).Seconds())
			return nil
		},
	}
}

func newAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <path>",
		Short: "Add a manual project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectPath, err := config.ExpandPath(args[0])
			if err != nil {
				return err
			}
			info, err := os.Stat(projectPath)
			if err != nil {
				return err
			}
			if !info.IsDir() {
				return fmt.Errorf("%s is not a directory", args[0])
			}
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			store, err := metadata.Load(paths.Metadata)
			if err != nil {
				return err
			}
			canonical, err := metadata.CanonicalPath(projectPath)
			if err != nil {
				return err
			}
			if store.Projects[canonical].Manual {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is already tracked by ovw.\n", displayPath(projectPath))
				return nil
			}
			entry := store.Projects[canonical]
			entry.Manual = true
			store.Projects[canonical] = entry
			if err := metadata.Write(paths.Metadata, store); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s to ovw.\n", displayPath(projectPath))
			return nil
		},
	}
}

func newVisibilityCommand(name string, hidden bool) *cobra.Command {
	return &cobra.Command{
		Use:   name + " <name-or-path>",
		Short: name + " a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			store, err := metadata.Load(paths.Metadata)
			if err != nil {
				return err
			}
			path, err := resolveProject(args[0], store)
			if err != nil {
				return err
			}
			entry := store.Projects[path]
			entry.Hidden = hidden
			store.Projects[path] = entry
			if err := metadata.Write(paths.Metadata, store); err != nil {
				return err
			}
			label := filepath.Base(path)
			if hidden {
				fmt.Fprintf(cmd.OutOrStdout(), "Hidden %s from ovw.\nNo files were deleted.\n", label)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is now visible in ovw.\n", label)
			}
			return nil
		},
	}
}

func resolveProject(target string, store metadata.Store) (string, error) {
	if expanded, err := config.ExpandPath(target); err == nil {
		if filepath.IsAbs(expanded) || target == "." || target == "~" || filepath.Clean(expanded) != filepath.Clean(target) {
			if canonical, err := metadata.CanonicalPath(expanded); err == nil {
				if _, ok := store.Projects[canonical]; ok {
					return canonical, nil
				}
				if _, err := os.Stat(canonical); err == nil {
					return canonical, nil
				}
			}
		}
	}
	matches := []string{}
	for path := range store.Projects {
		if filepath.Base(path) == target {
			matches = append(matches, path)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("Project %q is ambiguous; use full path", target)
	}
	return "", fmt.Errorf("project not found: %s", target)
}

func displayPath(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
