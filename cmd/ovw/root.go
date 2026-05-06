package ovw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ovw/internal/app"
	"ovw/internal/cache"
	"ovw/internal/config"
	"ovw/internal/metadata"
	"ovw/internal/project"
	"ovw/internal/scanner"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

func NewRootCommand() *cobra.Command {
	opts := app.Options{}
	cmd := &cobra.Command{
		Use:     "ovw",
		Short:   "A terminal overview for your local projects",
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
	cmd.AddCommand(newSetCommand())
	cmd.AddCommand(newUnsetCommand())
	cmd.AddCommand(newShowCommand())
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
		Short: "Rescan configured roots",
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
		Short: "Add a project manually",
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
	short := "Hide a project from ovw"
	if name == "remove" {
		short = "Hide a project from ovw without deleting files"
	}
	if !hidden {
		short = "Show a hidden project again"
	}
	return &cobra.Command{
		Use:   name + " <name-or-path>",
		Short: short,
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

func newSetCommand() *cobra.Command {
	var status string
	var note string
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Set project status or note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, cfg, store, err := commandState()
			if err != nil {
				return err
			}
			if status != "" && !statusAllowed(status, cfg.Statuses) {
				return fmt.Errorf("Unknown status: %s\nAvailable: %s", status, joinStatuses(cfg.Statuses))
			}
			path, err := resolveProjectWithConfig(args[0], cfg, store)
			if err != nil {
				return err
			}
			entry := store.Projects[path]
			if status != "" {
				entry.Status = status
			}
			if note != "" {
				entry.Note = note
			}
			store.Projects[path] = entry
			return metadata.Write(paths.Metadata, store)
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "set status")
	cmd.Flags().StringVar(&note, "note", "", "set note")
	return cmd
}

func newUnsetCommand() *cobra.Command {
	var clearStatus bool
	var clearNote bool
	cmd := &cobra.Command{
		Use:   "unset <name>",
		Short: "Clear project status or note",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, cfg, store, err := commandState()
			if err != nil {
				return err
			}
			path, err := resolveProjectWithConfig(args[0], cfg, store)
			if err != nil {
				return err
			}
			entry := store.Projects[path]
			if clearStatus {
				entry.Status = ""
			}
			if clearNote {
				entry.Note = ""
			}
			store.Projects[path] = entry
			return metadata.Write(paths.Metadata, store)
		},
	}
	cmd.Flags().BoolVar(&clearStatus, "status", false, "clear status")
	cmd.Flags().BoolVar(&clearNote, "note", false, "clear note")
	return cmd
}

func newShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, cfg, store, err := commandState()
			if err != nil {
				return err
			}
			path, err := resolveProjectWithConfig(args[0], cfg, store)
			if err != nil {
				return err
			}
			cacheStore, err := cache.Load(paths.Cache)
			if err != nil {
				return err
			}
			entry := store.Projects[path]
			enriched, _ := app.Enrich(scanner.Project{
				Name:   filepath.Base(path),
				Path:   path,
				Manual: entry.Manual,
				Hidden: entry.Hidden,
				Status: entry.Status,
				Note:   entry.Note,
			}, cfg, cacheStore, time.Now())
			fmt.Fprintf(cmd.OutOrStdout(), "Name      %s\n", filepath.Base(path))
			fmt.Fprintf(cmd.OutOrStdout(), "Path      %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "Stack     %s\n", strings.Join(enriched.Stack, ", "))
			fmt.Fprintf(cmd.OutOrStdout(), "Branch    %s\n", enriched.Activity.Branch)
			fmt.Fprintf(cmd.OutOrStdout(), "Activity  %s\n", detailActivity(enriched))
			fmt.Fprintf(cmd.OutOrStdout(), "Status    %s\n", enriched.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "Note      %s%s\n", enriched.Note.Display, noteSourceSuffix(enriched.Note.Source))
			fmt.Fprintf(cmd.OutOrStdout(), "Manual    %s\n", yesNo(enriched.Manual))
			return nil
		},
	}
}

func commandState() (config.FilePaths, config.Config, metadata.Store, error) {
	paths, err := config.Paths()
	if err != nil {
		return config.FilePaths{}, config.Config{}, metadata.Store{}, err
	}
	cfg, err := config.Load(paths.Config)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = config.Default()
		} else {
			return config.FilePaths{}, config.Config{}, metadata.Store{}, err
		}
	}
	store, err := metadata.Load(paths.Metadata)
	if err != nil {
		return config.FilePaths{}, config.Config{}, metadata.Store{}, err
	}
	return paths, cfg, store, nil
}

func resolveProjectWithConfig(target string, cfg config.Config, store metadata.Store) (string, error) {
	path, err := resolveProject(target, store)
	if err == nil {
		return path, nil
	}
	projects, scanErr := scanner.Scan(cfg, store)
	if scanErr != nil {
		return "", err
	}
	matches := []string{}
	for _, project := range projects {
		if project.Path == target || project.Name == target {
			matches = append(matches, project.Path)
		}
		if expanded, expandErr := config.ExpandPath(target); expandErr == nil {
			if canonical, canonicalErr := metadata.CanonicalPath(expanded); canonicalErr == nil && canonical == project.Path {
				matches = append(matches, project.Path)
			}
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("Project %q is ambiguous; use full path", target)
	}
	return "", err
}

func statusAllowed(status string, statuses []string) bool {
	for _, value := range statuses {
		if status == value {
			return true
		}
	}
	return false
}

func joinStatuses(statuses []string) string {
	out := ""
	for i, status := range statuses {
		if i > 0 {
			out += ", "
		}
		out += status
	}
	return out
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func detailActivity(project project.Project) string {
	activity := project.Activity
	if !activity.HasGit || !activity.HasCommits {
		return activity.Display
	}
	parts := []string{activity.LastCommitAge + " ago"}
	if activity.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("%d unpushed", activity.Unpushed))
	}
	if activity.Dirty {
		parts = append(parts, "dirty")
	}
	return strings.Join(parts, " · ")
}

func noteSourceSuffix(source string) string {
	if source == "" || source == "none" {
		return ""
	}
	return " (" + source + ")"
}
