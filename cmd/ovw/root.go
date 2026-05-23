package ovw

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"ovw/internal/app"
	"ovw/internal/buildinfo"
	"ovw/internal/config"
	"ovw/internal/project"
	"ovw/internal/projectview"
	"ovw/internal/render"
	"ovw/internal/tui"

	"github.com/spf13/cobra"
)

var (
	interactiveTerminal = streamsAreTerminal
	runTUI              = tui.RunWithOptions
	runFirstRunSetup    = tui.RunSetupWithOptions
	runConfigSetup      = tui.RunSetupWithConfig
)

func NewRootCommand() *cobra.Command {
	opts := app.Options{}
	cobra.EnableCommandSorting = false
	cmd := &cobra.Command{
		Use:           "ovw",
		Short:         "A terminal overview for your local projects.",
		Version:       buildinfo.Version,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		CompletionOptions: cobra.CompletionOptions{
			HiddenDefaultCmd: true,
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opts.Cwd = cwd
			opts.In = cmd.InOrStdin()
			opts.Out = cmd.OutOrStdout()
			opts.Err = cmd.ErrOrStderr()
			if len(args) == 1 {
				if isTemporarySessionArg(args[0]) && !opts.Open {
					opts.SessionRoot = cwd
					if !cmd.Flags().Changed("path") {
						opts.Path = cwd
					}
				} else {
					return runShow(cmd, args[0], opts.JSON, opts.Open)
				}
			}
			if opts.Open {
				return fmt.Errorf("pass a project name with --open")
			}
			if err := app.ValidateOptions(opts); err != nil {
				return err
			}
			if !opts.Plain && !opts.JSON && interactiveTerminal(opts.In, opts.Out) {
				if opts.SessionRoot == "" {
					setup, err := app.CheckConfig(opts)
					if err != nil {
						return err
					}
					if !setup.Exists {
						if err := runFirstRunSetup(opts, setup.Candidates); err != nil {
							if errors.Is(err, tui.ErrSetupCancelled) {
								return nil
							}
							return err
						}
					}
				}
				return runTUI(opts)
			}
			return app.Run(opts)
		},
	}
	cmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	cmd.SetUsageTemplate(rootUsageTemplate())
	cmd.Flags().BoolVar(&opts.Plain, "plain", false, "force plain table output")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output JSON for overview or project")
	cmd.Flags().BoolVarP(&opts.Open, "open", "o", false, "open project in editor")
	cmd.Flags().StringVar(&opts.Path, "path", "", "filter by project path")
	cmd.Flags().StringVar(&opts.Status, "status", "", "filter by status")
	cmd.Flags().BoolVar(&opts.Dirty, "dirty", false, "show dirty projects")
	cmd.Flags().BoolVar(&opts.Stale, "stale", false, "show stale projects")
	cmd.Flags().BoolVar(&opts.Untagged, "untagged", false, "show projects without status")
	cmd.Flags().BoolVar(&opts.Hidden, "hidden", false, "show hidden projects")
	cmd.Flags().StringVar(&opts.Sort, "sort", "", "sort by activity, updated, name, or status; optional :asc or :desc")
	cmd.Flags().BoolVar(&opts.Timing, "timing", false, "print scan timing to stderr")
	_ = cmd.Flags().MarkHidden("timing")
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

func isTemporarySessionArg(value string) bool {
	return filepath.Clean(strings.TrimSpace(value)) == "."
}

func streamsAreTerminal(in io.Reader, out io.Writer) bool {
	inFile, ok := in.(*os.File)
	if !ok {
		return false
	}
	outFile, ok := out.(*os.File)
	if !ok {
		return false
	}
	return isTerminalFile(inFile) && isTerminalFile(outFile)
}

func isTerminalFile(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func rootUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} [project] [flags]
  {{.CommandPath}} .
  {{.CommandPath}} [command]

Commands:
  add       Add a project path
  hide      Hide a project from ovw
  unhide    Show a hidden project again
  set       Set project metadata
  unset     Clear project metadata
  config    Manage ovw config

Flags:
  --json            output JSON for overview or project
  --plain           force plain table output
  -o, --open        open project in editor
  --path string     filter by project path
  --status string   filter by status
  --dirty           show dirty projects
  --stale           show stale projects
  --untagged        show projects without status
  --hidden          show hidden projects
  --sort string     sort by activity, updated, name, or status; optional :asc or :desc
  -h, --help        help for ovw
  -v, --version     version for ovw

Use "{{.CommandPath}} [command] --help" for more information about a command.
`
}

func addUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} <path>

Examples:
  {{.CommandPath}} ~/dev/myproject
`
}

func visibilityUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} <name-or-path>

Examples:
  {{.CommandPath}} myproject
  {{.CommandPath}} ~/dev/myproject
`
}

func configUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} <command>

Commands:
  path      Print config path
  edit      Edit config
  setup     Select project roots
  reset     Reset config

Use "{{.CommandPath}} <command> --help" for more information about a command.
`
}

func configPathUsageTemplate() string {
	return `Usage:
  {{.CommandPath}}
`
}

func configEditUsageTemplate() string {
	return `Usage:
  {{.CommandPath}}
`
}

func configSetupUsageTemplate() string {
	return `Usage:
  {{.CommandPath}}
`
}

func configResetUsageTemplate() string {
	return `Usage:
  {{.CommandPath}}
`
}

func newConfigCommand() *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ovw config",
		Long:  "Manage the ovw config file.",
	}
	configCmd.SetUsageTemplate(configUsageTemplate())
	pathCmd := &cobra.Command{
		Use:   "path",
		Short: "Print config path",
		Long:  "Print the path to config.toml.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), paths.Config)
			return nil
		},
	}
	pathCmd.SetUsageTemplate(configPathUsageTemplate())
	editCmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit config",
		Long:  "Open config.toml in your editor.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			return config.Edit(paths.Config)
		},
	}
	editCmd.SetUsageTemplate(configEditUsageTemplate())
	setupCmd := &cobra.Command{
		Use:   "setup",
		Short: "Select project roots",
		Long:  "Run the project root setup picker again.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opts := app.Options{
				Cwd: cwd,
				In:  cmd.InOrStdin(),
				Out: cmd.OutOrStdout(),
			}
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			cfg, err := config.Load(paths.Config)
			if err != nil {
				return err
			}
			candidates, err := config.RootCandidates(cwd)
			if err != nil {
				return err
			}
			candidates = mergeSetupCandidates(candidates, cfg.Roots)
			if err := runConfigSetup(opts, candidates, cfg.Roots, cfg); err != nil {
				if errors.Is(err, tui.ErrSetupCancelled) {
					return nil
				}
				return err
			}
			return nil
		},
	}
	setupCmd.SetUsageTemplate(configSetupUsageTemplate())
	resetCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset config",
		Long:  "Remove config.toml so the next ovw run starts setup again.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			paths, err := config.Paths()
			if err != nil {
				return err
			}
			if err := os.Remove(paths.Config); err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintln(cmd.OutOrStdout(), "Config already reset.")
					return nil
				}
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Config reset. Run ovw to set up project folders again.")
			return nil
		},
	}
	resetCmd.SetUsageTemplate(configResetUsageTemplate())
	configCmd.AddCommand(pathCmd)
	configCmd.AddCommand(editCmd)
	configCmd.AddCommand(setupCmd)
	configCmd.AddCommand(resetCmd)
	return configCmd
}

func mergeSetupCandidates(candidates, roots []string) []string {
	seen := make(map[string]bool, len(candidates)+len(roots))
	merged := make([]string, 0, len(candidates)+len(roots))
	for _, value := range append(append([]string{}, candidates...), roots...) {
		if value == "" || seen[value] {
			continue
		}
		if hasSetupAncestorCandidate(merged, value) {
			continue
		}
		seen[value] = true
		merged = append(merged, value)
	}
	return merged
}

func hasSetupAncestorCandidate(candidates []string, value string) bool {
	for _, candidate := range candidates {
		if setupCandidateContains(candidate, value) {
			return true
		}
	}
	return false
}

func setupCandidateContains(parent, child string) bool {
	parent = normalizeSetupCandidate(parent)
	child = normalizeSetupCandidate(child)
	return child != parent && strings.HasPrefix(child, parent+"/")
}

func normalizeSetupCandidate(value string) string {
	if expanded, err := config.ExpandPath(value); err == nil {
		value = expanded
	}
	return strings.TrimRight(filepath.ToSlash(value), "/")
}

func newAddCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <path>",
		Short: "Add a project path",
		Long:  "Add one project path to ovw.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			projectPath, err := config.ExpandPath(args[0])
			if err != nil {
				return err
			}
			result, err := app.AddProject(args[0])
			if err != nil {
				return err
			}
			if result.AlreadyTracked {
				fmt.Fprintf(cmd.OutOrStdout(), "%s is already tracked by ovw.\n", displayPath(projectPath))
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Added %s to ovw.\n", displayPath(projectPath))
			return nil
		},
	}
	cmd.SetUsageTemplate(addUsageTemplate())
	return cmd
}

func newVisibilityCommand(name string, hidden bool) *cobra.Command {
	short := "Hide a project from ovw"
	long := "Hide one project from ovw without deleting files."
	if name == "remove" {
		short = "Hide a project from ovw without deleting files"
	}
	if !hidden {
		short = "Show a hidden project again"
		long = "Show one hidden project in ovw again."
	}
	hideFromHelp := name == "remove"
	cmd := &cobra.Command{
		Use:    name + " <name-or-path>",
		Short:  short,
		Long:   long,
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
	cmd.SetUsageTemplate(visibilityUsageTemplate())
	return cmd
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
	var pin bool
	var scriptValues []string
	var fieldValues []string
	cmd := &cobra.Command{
		Use:   "set <project>",
		Short: "Set project metadata",
		Long:  "Set project metadata: status, note, pin, script, or field.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if status == "" && note == "" && !pin && len(scriptValues) == 0 && len(fieldValues) == 0 {
				return fmt.Errorf("nothing to set; pass --status, --note, --pin, --script, or --field")
			}
			update := app.MetadataUpdate{}
			if status != "" {
				update.Status = &status
			}
			if note != "" {
				update.Note = &note
			}
			if pin {
				pinned := true
				update.Pinned = &pinned
			}
			scripts, err := parseScriptAssignments(scriptValues)
			if err != nil {
				return err
			}
			if len(scripts) > 0 {
				update.Scripts = scripts
			}
			fields, err := parseFieldAssignments(fieldValues)
			if err != nil {
				return err
			}
			if len(fields) > 0 {
				cfg, err := loadCLIConfig()
				if err != nil {
					return err
				}
				if err := validateFieldAssignments(cfg.Fields, fields); err != nil {
					return err
				}
				update.Fields = fields
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
			if pin {
				fmt.Fprintln(cmd.OutOrStdout(), "Pinned  yes")
			}
			for _, script := range scriptValues {
				name, command, _ := strings.Cut(script, "=")
				fmt.Fprintf(cmd.OutOrStdout(), "Script  %s = %s\n", name, command)
			}
			for _, field := range fieldValues {
				name, value, _ := strings.Cut(field, "=")
				name = strings.TrimSpace(name)
				value = strings.TrimSpace(value)
				if normalized, ok := fields[name]; ok && normalized != nil {
					value = *normalized
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Field   %s = %s\n", name, value)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "set status")
	cmd.Flags().StringVar(&note, "note", "", "set note")
	cmd.Flags().BoolVar(&pin, "pin", false, "pin project")
	cmd.Flags().StringArrayVar(&scriptValues, "script", nil, "set script as name=command")
	cmd.Flags().StringArrayVar(&fieldValues, "field", nil, "set custom field as field=value")
	cmd.SetUsageTemplate(setUsageTemplate())
	return cmd
}

func newUnsetCommand() *cobra.Command {
	var clearStatus bool
	var clearNote bool
	var clearPin bool
	var scriptNames []string
	var fieldNames []string
	cmd := &cobra.Command{
		Use:   "unset <project>",
		Short: "Clear project metadata",
		Long:  "Clear project metadata: status, note, pin, script, or field.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !clearStatus && !clearNote && !clearPin && len(scriptNames) == 0 && len(fieldNames) == 0 {
				return fmt.Errorf("nothing to unset; pass --status, --note, --pin, --script, or --field")
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
			if clearPin {
				pinned := false
				update.Pinned = &pinned
			}
			scripts, err := parseScriptNames(scriptNames)
			if err != nil {
				return err
			}
			if len(scripts) > 0 {
				update.Scripts = scripts
			}
			fields, err := parseFieldNames(fieldNames)
			if err != nil {
				return err
			}
			if len(fields) > 0 {
				cfg, err := loadCLIConfig()
				if err != nil {
					return err
				}
				if err := validateFieldNames(cfg.Fields, fields); err != nil {
					return err
				}
				update.Fields = fields
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
			if clearPin {
				fmt.Fprintln(cmd.OutOrStdout(), "Pin cleared.")
			}
			for _, script := range scriptNames {
				fmt.Fprintf(cmd.OutOrStdout(), "Script %s cleared.\n", script)
			}
			for _, field := range fieldNames {
				fmt.Fprintf(cmd.OutOrStdout(), "Field %s cleared.\n", strings.TrimSpace(field))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&clearStatus, "status", false, "clear status")
	cmd.Flags().BoolVar(&clearNote, "note", false, "clear note")
	cmd.Flags().BoolVar(&clearPin, "pin", false, "unpin project")
	cmd.Flags().StringArrayVar(&scriptNames, "script", nil, "clear script by name")
	cmd.Flags().StringArrayVar(&fieldNames, "field", nil, "clear custom field by id")
	cmd.SetUsageTemplate(unsetUsageTemplate())
	return cmd
}

func parseScriptAssignments(values []string) (map[string]*string, error) {
	scripts := map[string]*string{}
	for _, value := range values {
		name, command, ok := strings.Cut(value, "=")
		name = strings.TrimSpace(name)
		command = strings.TrimSpace(command)
		if !ok || name == "" || command == "" {
			return nil, fmt.Errorf("invalid script %q: expected name=command", value)
		}
		scriptCommand := command
		scripts[name] = &scriptCommand
	}
	return scripts, nil
}

func parseScriptNames(values []string) (map[string]*string, error) {
	scripts := map[string]*string{}
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			return nil, fmt.Errorf("invalid script name: cannot be empty")
		}
		scripts[name] = nil
	}
	return scripts, nil
}

func parseFieldAssignments(values []string) (map[string]*string, error) {
	fields := map[string]*string{}
	for _, value := range values {
		name, fieldValue, ok := strings.Cut(value, "=")
		name = strings.TrimSpace(name)
		fieldValue = strings.TrimSpace(fieldValue)
		if !ok || name == "" || fieldValue == "" {
			return nil, fmt.Errorf("invalid field %q: expected field=value", value)
		}
		valueCopy := fieldValue
		fields[name] = &valueCopy
	}
	return fields, nil
}

func parseFieldNames(values []string) (map[string]*string, error) {
	fields := map[string]*string{}
	for _, value := range values {
		name := strings.TrimSpace(value)
		if name == "" {
			return nil, fmt.Errorf("invalid field name: cannot be empty")
		}
		fields[name] = nil
	}
	return fields, nil
}

func loadCLIConfig() (config.Config, error) {
	paths, err := config.Paths()
	if err != nil {
		return config.Config{}, err
	}
	return config.Load(paths.Config)
}

func validateFieldAssignments(defs []config.FieldConfig, values map[string]*string) error {
	for id, value := range values {
		def, ok := fieldDefByID(defs, id)
		if !ok {
			return unknownFieldError(id)
		}
		if value == nil {
			continue
		}
		normalized, err := normalizeFieldValue(def, *value)
		if err != nil {
			return err
		}
		*value = normalized
	}
	return nil
}

func validateFieldNames(defs []config.FieldConfig, values map[string]*string) error {
	for id := range values {
		if _, ok := fieldDefByID(defs, id); !ok {
			return unknownFieldError(id)
		}
	}
	return nil
}

func fieldDefByID(defs []config.FieldConfig, id string) (config.FieldConfig, bool) {
	for _, def := range defs {
		if def.ID == id {
			return def, true
		}
	}
	return config.FieldConfig{}, false
}

func normalizeFieldValue(def config.FieldConfig, value string) (string, error) {
	switch def.Type {
	case "text":
		return value, nil
	case "select":
		for _, option := range def.Options {
			if value == option {
				return value, nil
			}
		}
		return "", fmt.Errorf("invalid value %q for field %q; expected %s", value, def.ID, humanList(def.Options))
	case "checkbox":
		switch strings.ToLower(value) {
		case "true", "yes", "on", "1":
			return "true", nil
		case "false", "no", "off", "0":
			return "false", nil
		default:
			return "", fmt.Errorf("invalid value %q for field %q; expected true or false", value, def.ID)
		}
	default:
		return "", fmt.Errorf("invalid field %q: unsupported type %q", def.ID, def.Type)
	}
}

func unknownFieldError(id string) error {
	return fmt.Errorf("unknown field %q; add it in Settings > Fields or ovw config edit", id)
}

func humanList(values []string) string {
	switch len(values) {
	case 0:
		return "a configured option"
	case 1:
		return values[0]
	case 2:
		return values[0] + " or " + values[1]
	default:
		return strings.Join(values[:len(values)-1], ", ") + ", or " + values[len(values)-1]
	}
}

func setUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} <project> --status <status>
  {{.CommandPath}} <project> --note <note>
  {{.CommandPath}} <project> --pin
  {{.CommandPath}} <project> --script <name=command>
  {{.CommandPath}} <project> --field <field=value>
  {{.CommandPath}} <project> --status <status> --note <note>

Examples:
  {{.CommandPath}} myproject --status blocked
  {{.CommandPath}} myproject --note "currently working on it"
  {{.CommandPath}} myproject --pin
  {{.CommandPath}} myproject --script run="go run ."
  {{.CommandPath}} myproject --field owner=roie
  {{.CommandPath}} myproject --status shipped --note "released v1"

Flags:
  --status string   set status
  --note string     set note
  --pin             pin project
  --script string   set script as name=command
  --field string    set custom field as field=value
  -h, --help        help for set
`
}

func unsetUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} <project> --status
  {{.CommandPath}} <project> --note
  {{.CommandPath}} <project> --pin
  {{.CommandPath}} <project> --script <name>
  {{.CommandPath}} <project> --field <field>
  {{.CommandPath}} <project> --status --note

Examples:
  {{.CommandPath}} myproject --status
  {{.CommandPath}} myproject --note
  {{.CommandPath}} myproject --pin
  {{.CommandPath}} myproject --script run
  {{.CommandPath}} myproject --field owner

Flags:
  --status   clear status
  --note     clear note
  --pin      unpin project
  --script string   clear script by name
  --field string    clear custom field by id
  -h, --help  help for unset
`
}

func newShowCommand() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:    "show <name>",
		Short:  "Show project details",
		Long:   "Show details for one project.",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runShow(cmd, args[0], jsonOutput, false)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "output JSON")
	cmd.SetUsageTemplate(showUsageTemplate())
	return cmd
}

func showUsageTemplate() string {
	return `Usage:
  {{.CommandPath}} <project>
  {{.CommandPath}} <project> --json

Flags:
  --json      output JSON
  -h, --help  help for show
`
}

func runShow(cmd *cobra.Command, target string, jsonOutput, openProject bool) error {
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
		if openProject {
			return fmt.Errorf("choose only one action: --json or --open")
		}
		return render.ProjectJSON(out, enriched)
	}
	if openProject {
		if err := tui.OpenEditor(state.Config.Editor, path); err != nil {
			return err
		}
		fmt.Fprintf(out, "Opened %s.\n", filepath.Base(path))
		return nil
	}
	fmt.Fprintf(out, "%s\n", filepath.Base(path))
	if subtitle := projectview.Subtitle(enriched); subtitle != "" {
		fmt.Fprintf(out, "%s\n\n", subtitle)
	}
	fields := projectview.Fields(enriched, projectview.Options{
		Path:     displayPath,
		Time:     showTime,
		Activity: projectview.DetailActivity,
	})
	for _, field := range fields {
		fmt.Fprintf(out, "%-9s %s\n", field.Label, field.Value)
	}
	return nil
}

func showTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format("2006-01-02 15:04")
}

func detailActivity(project project.Project) string {
	return projectview.DetailActivity(project)
}
