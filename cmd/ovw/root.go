package ovw

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ovw/internal/app"
	"ovw/internal/config"
	ovwformat "ovw/internal/format"
	"ovw/internal/metadata"
	"ovw/internal/project"
	"ovw/internal/render"

	"github.com/spf13/cobra"
)

const version = "0.1.0"

func NewRootCommand() *cobra.Command {
	opts := app.Options{}
	cobra.EnableCommandSorting = false
	cmd := &cobra.Command{
		Use:           "ovw",
		Short:         "A terminal overview for your local projects",
		Version:       version,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
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
	cmd.AddCommand(newConfigCommand())
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
  config    Manage ovw config

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
			result, err := app.SetProjectHidden(args[0], hidden)
			if err != nil {
				return err
			}
			label := filepath.Base(result.Path)
			if hidden {
				fmt.Fprintf(cmd.OutOrStdout(), "Hidden %s from ovw.\nNo files were deleted.\n", label)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is now visible in ovw.\n", label)
			}
			return nil
		},
	}
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
			if status == "" && note == "" {
				return fmt.Errorf("nothing to set; pass --status or --note")
			}
			update := app.MetadataUpdate{}
			if status != "" {
				update.Status = &status
			}
			if note != "" {
				update.Note = &note
			}
			result, err := app.UpdateProjectMetadata(args[0], update)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s.\n", filepath.Base(result.Path))
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
			if !clearStatus && !clearNote {
				return fmt.Errorf("nothing to unset; pass --status or --note")
			}
			update := app.MetadataUpdate{}
			if clearStatus {
				empty := ""
				update.Status = &empty
			}
			if clearNote {
				empty := ""
				update.Note = &empty
			}
			result, err := app.UpdateProjectMetadata(args[0], update)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated %s.\n", filepath.Base(result.Path))
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
	state, err := app.LoadState()
	if err != nil {
		return err
	}
	path, err := app.ResolveProject(target, state.Config, state.Store)
	if err != nil {
		return err
	}
	enriched := app.ProjectFromPath(path, state.Config, state.Store, time.Now())
	out := cmd.OutOrStdout()
	if jsonOutput {
		return render.ProjectJSON(out, enriched)
	}
	fmt.Fprintf(out, "%s\n", filepath.Base(path))
	fmt.Fprintf(out, "Path      %s\n", displayPath(path))
	fmt.Fprintf(out, "Stack     %s\n", strings.Join(enriched.Stack, ", "))
	fmt.Fprintf(out, "Manager   %s\n", managerDisplay(enriched.Managers))
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

func showTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}

func managerDisplay(managers []string) string {
	if len(managers) == 0 {
		return "—"
	}
	return strings.Join(managers, ", ")
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
