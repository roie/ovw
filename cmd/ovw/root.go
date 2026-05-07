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
	ovwformat "ovw/internal/format"
	"ovw/internal/metadata"
	"ovw/internal/project"
	"ovw/internal/render"
	"ovw/internal/scanner"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

func NewRootCommand() *cobra.Command {
	opts := app.Options{}
	cobra.EnableCommandSorting = false
	cmd := &cobra.Command{
		Use:     "ovw",
		Short:   "A terminal overview for your local projects",
		Version: version,
		Args:    cobra.MaximumNArgs(1),
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				return runShow(cmd, args[0], opts.JSON)
			}
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
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	cmd.SetUsageTemplate(rootUsageTemplate())
	cmd.Flags().BoolVar(&opts.Plain, "plain", false, "force plain table output")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output JSON for overview or project")
	cmd.Flags().StringVar(&opts.Status, "status", "", "filter by manual status")
	cmd.Flags().BoolVar(&opts.Dirty, "dirty", false, "show dirty projects")
	cmd.Flags().BoolVar(&opts.Stale, "stale", false, "show stale projects")
	cmd.Flags().BoolVar(&opts.Untagged, "untagged", false, "show projects without manual status")
	cmd.Flags().BoolVar(&opts.Hidden, "hidden", false, "show hidden projects")
	cmd.Flags().StringVar(&opts.Sort, "sort", "", "sort by activity, name, or status")
	cmd.AddCommand(newAddCommand())
	cmd.AddCommand(newVisibilityCommand("hide", true))
	cmd.AddCommand(newVisibilityCommand("remove", true))
	cmd.AddCommand(newVisibilityCommand("unhide", false))
	cmd.AddCommand(newSetCommand())
	cmd.AddCommand(newUnsetCommand())
	cmd.AddCommand(newScanCommand())
	cmd.AddCommand(newConfigCommand())
	cmd.AddCommand(newCacheCommand())
	cmd.AddCommand(newShowCommand())
	return cmd
}

func rootUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} [project] [flags]
  {{.CommandPath}} [command]

Commands:
  add       Add a project manually
  hide      Hide a project from ovw
  unhide    Show a hidden project again
  set       Set project status or note
  unset     Clear project status or note
  scan      Rescan configured roots
  config    Manage ovw config
  cache     Manage ovw cache

Flags:
  --json            output JSON for overview or project
  --plain           force plain table output
  --status string   filter by manual status
  --dirty           show dirty projects
  --stale           show stale projects
  --untagged        show projects without manual status
  --hidden          show hidden projects
  --sort string     sort by activity, name, or status
  -h, --help        help for ovw
  -v, --version     version for ovw

Use "{{.CommandPath}} [command] --help" for more information about a command.
`
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
	hideFromHelp := name == "remove"
	return &cobra.Command{
		Use:    name + " <name-or-path>",
		Short:  short,
		Hidden: hideFromHelp,
		Args:   cobra.ExactArgs(1),
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
		home, homeErr := os.UserHomeDir()
		if homeErr == nil {
			if filepath.Clean(abs) == filepath.Clean(home) {
				return "~"
			}
			if rel, relErr := filepath.Rel(home, abs); relErr == nil && rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return filepath.Join("~", rel)
			}
		}
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
			if err := metadata.Write(paths.Metadata, store); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s.\n", filepath.Base(path))
			if status != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Status  %s\n", status)
			}
			if note != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Note    %s\n", note)
			}
			return nil
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
			if err := metadata.Write(paths.Metadata, store); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s.\n", filepath.Base(path))
			if clearStatus {
				fmt.Fprintln(cmd.OutOrStdout(), "Status cleared.")
			}
			if clearNote {
				fmt.Fprintln(cmd.OutOrStdout(), "Note cleared.")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&clearStatus, "status", false, "clear status")
	cmd.Flags().BoolVar(&clearNote, "note", false, "clear note")
	return cmd
}

func newShowCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:    "show <name>",
		Short:  "Show project details",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cmd, args[0], jsonOutput)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	return cmd
}

func runShow(cmd *cobra.Command, target string, jsonOutput bool) error {
	paths, cfg, store, err := commandState()
	if err != nil {
		return err
	}
	path, err := resolveProjectWithConfig(target, cfg, store)
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
	out := cmd.OutOrStdout()
	if jsonOutput {
		return render.ProjectJSON(out, enriched)
	}
	fmt.Fprintf(out, "%s\n", filepath.Base(path))
	fmt.Fprintf(out, "Path      %s\n", displayPath(path))
	fmt.Fprintf(out, "Stack     %s\n", strings.Join(enriched.Stack, ", "))
	fmt.Fprintf(out, "Status    %s\n", ovwformat.TagDisplay(enriched.Tags))
	fmt.Fprintf(out, "Note      %s\n", enriched.Note.Display)
	if enriched.Activity.HasGit && enriched.Activity.Branch != "" {
		fmt.Fprintf(out, "Branch    %s\n", enriched.Activity.Branch)
	}
	fmt.Fprintf(out, "Activity  %s\n", detailActivity(enriched))
	if enriched.Activity.HasGit && !enriched.Activity.LastCommitAt.IsZero() {
		fmt.Fprintf(out, "Updated   %s\n", showTime(enriched.Activity.LastCommitAt))
	}
	return nil
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

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func showTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}

func detailActivity(project project.Project) string {
	activity := project.Activity
	if !activity.HasGit || !activity.HasCommits {
		return activity.Display
	}
	age := activity.LastCommitAge
	if age != "now" {
		age += " ago"
	}
	parts := []string{age}
	if activity.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("%d unpushed", activity.Unpushed))
	}
	return strings.Join(parts, " · ")
}

func noteSourceSuffix(source string) string {
	if source == "" || source == "none" {
		return ""
	}
	return " (" + source + ")"
}
