package ovw

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"ovw/internal/app"
	"ovw/internal/config"
	"ovw/internal/format"
	"ovw/internal/project"
	"ovw/internal/tui"
)

func TestHelpIncludesUsage(t *testing.T) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte("Usage:")) {
		t.Fatalf("help output missing Usage: %q", got)
	}
	if !bytes.Contains([]byte(got), []byte("ovw")) {
		t.Fatalf("help output missing command name: %q", got)
	}
}

func TestHelpTextDescriptions(t *testing.T) {
	cmd := NewRootCommand()
	if cmd.Short != "A terminal overview for your local projects." {
		t.Fatalf("Short = %q", cmd.Short)
	}
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--help"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"A terminal overview for your local projects.",
		"ovw [project] [flags]",
		"Commands:",
		"add       Add a project path",
		"hide      Hide a project from ovw",
		"unhide    Show a hidden project again",
		"set       Set project status or note",
		"unset     Clear project status or note",
		"config    Manage ovw config",
		"--json            output JSON for overview or project",
		"--status string   filter by status",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("help output missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "show        Show project details") {
		t.Fatalf("help output should hide show alias:\n%s", got)
	}
	if strings.Contains(got, "remove      Hide a project from ovw without deleting files") {
		t.Fatalf("help output should hide remove alias:\n%s", got)
	}
	if strings.Contains(got, "completion  Generate") {
		t.Fatalf("help output should hide completion command:\n%s", got)
	}
	if strings.Contains(got, "cache     Manage ovw cache") {
		t.Fatalf("help output should not list cache command:\n%s", got)
	}
	if strings.Contains(got, "scan      Rescan configured roots") {
		t.Fatalf("help output should not list scan command:\n%s", got)
	}
	if strings.Contains(got, "help        Help about any command") {
		t.Fatalf("help output should not list help command:\n%s", got)
	}
	if strings.Contains(got, "Examples:") || strings.Contains(got, "ovw myproject") {
		t.Fatalf("help output should not include examples:\n%s", got)
	}
	mustAppearInOrder(t, got, []string{
		"add       Add a project path",
		"hide      Hide a project from ovw",
		"unhide    Show a hidden project again",
		"set       Set project status or note",
		"unset     Clear project status or note",
		"config    Manage ovw config",
	})
	mustAppearInOrder(t, got, []string{
		"--json            output JSON for overview or project",
		"--plain           force plain table output",
		"--status string   filter by status",
		"--dirty           show dirty projects",
		"--stale           show stale projects",
		"--untagged        show projects without status",
		"--hidden          show hidden projects",
		"--sort string     sort by activity, name, or status; optional :asc or :desc",
		"-h, --help        help for ovw",
		"-v, --version     version for ovw",
	})
}

func TestVersionOutput(t *testing.T) {
	out, err := executeCommand([]string{"-v"})
	if err != nil {
		t.Fatalf("Execute(-v) error = %v", err)
	}
	if out != "ovw 1.0.0\n" {
		t.Fatalf("version output = %q", out)
	}
}

func TestSetHelpShowsStatusAndNoteUsage(t *testing.T) {
	out, err := executeCommand([]string{"set", "--help"})
	if err != nil {
		t.Fatalf("Execute(set --help) error = %v", err)
	}
	for _, want := range []string{
		"Set a short status or note for one project.",
		"ovw set <project> --status <status>",
		"ovw set <project> --note <note>",
		"ovw set myproject --status blocked",
		"ovw set myproject --note \"currently working on it\"",
		"--status string   set status",
		"--note string     set note",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("set help missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--json") || strings.Contains(out, "Commands:") {
		t.Fatalf("set help should not show root help:\n%s", out)
	}
}

func TestUnsetHelpShowsStatusAndNoteUsage(t *testing.T) {
	out, err := executeCommand([]string{"unset", "--help"})
	if err != nil {
		t.Fatalf("Execute(unset --help) error = %v", err)
	}
	for _, want := range []string{
		"Clear a project status or note.",
		"ovw unset <project> --status",
		"ovw unset <project> --note",
		"ovw unset myproject --status",
		"ovw unset myproject --note",
		"--status   clear status",
		"--note     clear note",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("unset help missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "--json") || strings.Contains(out, "Commands:") {
		t.Fatalf("unset help should not show root help:\n%s", out)
	}
}

func TestProjectCommandHelpIsFocused(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "add",
			args: []string{"add", "--help"},
			want: []string{
				"Add one project path to ovw.",
				"ovw add <path>",
				"ovw add ~/dev/myproject",
			},
		},
		{
			name: "hide",
			args: []string{"hide", "--help"},
			want: []string{
				"Hide one project from ovw without deleting files.",
				"ovw hide <name-or-path>",
				"ovw hide myproject",
			},
		},
		{
			name: "unhide",
			args: []string{"unhide", "--help"},
			want: []string{
				"Show one hidden project in ovw again.",
				"ovw unhide <name-or-path>",
				"ovw unhide myproject",
			},
		},
		{
			name: "show",
			args: []string{"show", "--help"},
			want: []string{
				"Show details for one project.",
				"ovw show <project>",
				"ovw show <project> --json",
				"--json      output JSON",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := executeCommand(tc.args)
			if err != nil {
				t.Fatalf("Execute(%v) error = %v", tc.args, err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("%s help missing %q:\n%s", tc.name, want, out)
				}
			}
			if strings.Contains(out, "--dirty") || strings.Contains(out, "Commands:") {
				t.Fatalf("%s help should not show root help:\n%s", tc.name, out)
			}
			if tc.name != "show" && strings.Contains(out, "Flags:") {
				t.Fatalf("%s help should not show help-only flags:\n%s", tc.name, out)
			}
		})
	}
}

func TestConfigHelpIsFocused(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "config",
			args: []string{"config", "--help"},
			want: []string{
				"Manage the ovw config file.",
				"ovw config <command>",
				"path      Print config path",
				"edit      Edit config",
				"setup     Select project roots",
			},
		},
		{
			name: "config path",
			args: []string{"config", "path", "--help"},
			want: []string{
				"Print the path to config.toml.",
				"ovw config path",
			},
		},
		{
			name: "config edit",
			args: []string{"config", "edit", "--help"},
			want: []string{
				"Open config.toml in your editor.",
				"ovw config edit",
			},
		},
		{
			name: "config setup",
			args: []string{"config", "setup", "--help"},
			want: []string{
				"Run the project root setup picker again.",
				"ovw config setup",
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := executeCommand(tc.args)
			if err != nil {
				t.Fatalf("Execute(%v) error = %v", tc.args, err)
			}
			for _, want := range tc.want {
				if !strings.Contains(out, want) {
					t.Fatalf("%s help missing %q:\n%s", tc.name, want, out)
				}
			}
			if strings.Contains(out, "--dirty") || strings.Contains(out, "add       Add a project path") {
				t.Fatalf("%s help should not show root help:\n%s", tc.name, out)
			}
			if strings.Contains(out, "Flags:") {
				t.Fatalf("%s help should not show help-only flags:\n%s", tc.name, out)
			}
		})
	}
}

func TestRuntimeErrorsDoNotPrintUsage(t *testing.T) {
	out, err := executeCommand([]string{"missing-project"})
	if err == nil {
		t.Fatal("expected missing project error")
	}
	if strings.Contains(out, "Usage:") || strings.Contains(out, "Commands:") {
		t.Fatalf("runtime error printed usage:\n%s", out)
	}
}

func TestInteractiveBareCommandRunsTUI(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	configForTest(t, root)
	tuiCalled := false
	withTerminalRouting(t, true, func(opts app.Options) error {
		tuiCalled = true
		return nil
	})

	out, err := executeCommand(nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !tuiCalled {
		t.Fatal("expected TUI runner to be called")
	}
	if out != "" {
		t.Fatalf("interactive TUI route wrote plain output: %q", out)
	}
}

func TestNonInteractiveBareCommandUsesPlainTable(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	configForTest(t, root)
	withTerminalRouting(t, false, func(opts app.Options) error {
		t.Fatal("TUI runner should not be called")
		return nil
	})

	out, err := executeCommand(nil)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out, "app") {
		t.Fatalf("plain output missing project: %q", out)
	}
}

func TestInteractivePlainJSONAndProjectSkipTUI(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	configForTest(t, root)
	withTerminalRouting(t, true, func(opts app.Options) error {
		t.Fatal("TUI runner should not be called")
		return nil
	})

	for _, args := range [][]string{
		{"--plain"},
		{"--json"},
		{"app"},
	} {
		if _, err := executeCommand(args); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
	}
}

func TestInteractiveRootPassesFlagsToTUI(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	configForTest(t, root)
	var got app.Options
	withTerminalRouting(t, true, func(opts app.Options) error {
		got = opts
		return nil
	})

	if _, err := executeCommand([]string{"--dirty", "--sort", "name"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !got.Dirty || got.Sort != "name" {
		t.Fatalf("TUI options = %#v", got)
	}
}

func TestInteractiveFirstRunRunsSetupBeforeTUI(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "dev")
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	previousSetup := runFirstRunSetup
	defer func() { runFirstRunSetup = previousSetup }()
	var setupCandidates []string
	setupCalled := false
	tuiCalled := false
	withTerminalRouting(t, true, func(opts app.Options) error {
		tuiCalled = true
		return nil
	})
	runFirstRunSetup = func(opts app.Options, candidates []string) error {
		setupCalled = true
		setupCandidates = candidates
		return nil
	}

	if _, err := executeCommand(nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !setupCalled {
		t.Fatal("expected first-run setup to be called")
	}
	if len(setupCandidates) == 0 || setupCandidates[0] != "~/dev" {
		t.Fatalf("setup candidates = %#v, want first ~/dev", setupCandidates)
	}
	if !tuiCalled {
		t.Fatal("expected TUI runner after setup")
	}
}

func TestInteractiveFirstRunCancelSkipsTUI(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "dev")
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	previousSetup := runFirstRunSetup
	defer func() { runFirstRunSetup = previousSetup }()
	withTerminalRouting(t, true, func(opts app.Options) error {
		t.Fatal("TUI runner should not be called after cancelled setup")
		return nil
	})
	runFirstRunSetup = func(opts app.Options, candidates []string) error {
		return tui.ErrSetupCancelled
	}

	if _, err := executeCommand(nil); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
}

func TestConfigPathCommand(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"config", "path"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	want := filepath.Join(home, ".config", "ovw", "config.toml") + "\n"
	if out.String() != want {
		t.Fatalf("config path = %q, want %q", out.String(), want)
	}
}

func TestConfigSubcommandsRejectExtraArgs(t *testing.T) {
	for _, args := range [][]string{
		{"config", "path", "extra"},
		{"config", "edit", "extra"},
		{"config", "setup", "extra"},
	} {
		if _, err := executeCommand(args); err == nil {
			t.Fatalf("Execute(%v) error = nil, want extra arg error", args)
		}
	}
}

func TestConfigSetupRunsPickerWithCurrentRoots(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "dev")
	current := filepath.Join(home, "work")
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	configForTest(t, current)
	previousSetup := runConfigSetup
	defer func() { runConfigSetup = previousSetup }()
	var gotCandidates []string
	var gotRoots []string
	runConfigSetup = func(opts app.Options, candidates, roots []string) error {
		gotCandidates = append([]string{}, candidates...)
		gotRoots = append([]string{}, roots...)
		return nil
	}

	if _, err := executeCommand([]string{"config", "setup"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(gotRoots) != 1 || gotRoots[0] != current {
		t.Fatalf("roots = %#v, want %q", gotRoots, current)
	}
	if !containsString(gotCandidates, current) {
		t.Fatalf("candidates = %#v, want current root", gotCandidates)
	}
}

func TestConfigSetupDoesNotDuplicateNestedCurrentRoots(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "dev")
	current := filepath.Join(root, "extensions")
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	if err := os.MkdirAll(current, 0o755); err != nil {
		t.Fatal(err)
	}
	configForTest(t, current)
	previousSetup := runConfigSetup
	defer func() { runConfigSetup = previousSetup }()
	var gotCandidates []string
	runConfigSetup = func(opts app.Options, candidates, roots []string) error {
		gotCandidates = append([]string{}, candidates...)
		return nil
	}

	if _, err := executeCommand([]string{"config", "setup"}); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if containsString(gotCandidates, current) {
		t.Fatalf("nested current root should not be top-level candidate: %#v", gotCandidates)
	}
	if !containsString(gotCandidates, "~/dev") {
		t.Fatalf("candidates = %#v, want ~/dev", gotCandidates)
	}
}

func TestCacheCommandIsRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CACHE_HOME", "")

	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"cache", "clear"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected cache command error")
	}
}

func TestAddHideUnhideRemoveCommands(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)

	runCommand(t, []string{"add", project})
	cfg, err := config.Load(filepath.Join(home, ".config", "ovw", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != project {
		t.Fatalf("roots after add = %#v", cfg.Roots)
	}

	out := runCommand(t, []string{"hide", project})
	if !bytes.Contains([]byte(out), []byte("No files were deleted.")) {
		t.Fatalf("hide output = %q", out)
	}
	if _, err := os.Stat(project); err != nil {
		t.Fatalf("project was deleted: %v", err)
	}

	runCommand(t, []string{"unhide", project})
	metaPath := filepath.Join(home, ".local", "share", "ovw", "projects.json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte(`"hidden": true`)) {
		t.Fatalf("metadata after unhide = %s", string(data))
	}

	runCommand(t, []string{"remove", project})
	data, err = os.ReadFile(metaPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(data, []byte(`"hidden": true`)) {
		t.Fatalf("metadata after remove = %s", string(data))
	}
}

func TestAddExpandsQuotedHomePathAndDisplaysShortPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := filepath.Join(home, "dev", "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	out := runCommand(t, []string{"add", "~/dev/custom"})
	if !strings.Contains(out, "Added ~/dev/custom to ovw.") {
		t.Fatalf("add output = %q", out)
	}
	cfg, err := config.Load(filepath.Join(home, ".config", "ovw", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Roots) != 1 || cfg.Roots[0] != "~/dev/custom" {
		t.Fatalf("config roots = %#v, want ~/dev/custom", cfg.Roots)
	}
}

func TestHideCanTargetScannedProjectByName(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "scanned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scanned", "go.mod"), []byte("module scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)

	out := runCommand(t, []string{"hide", "scanned"})
	if !strings.Contains(out, "Hidden scanned from ovw.") || !strings.Contains(out, "No files were deleted.") {
		t.Fatalf("hide output = %q", out)
	}
	overview := runCommand(t, []string{"--plain"})
	if strings.Contains(overview, "\nscanned  ") {
		t.Fatalf("hidden scanned project still visible:\n%s", overview)
	}
	out = runCommand(t, []string{"unhide", "scanned"})
	if !strings.Contains(out, "scanned is now visible in ovw.") {
		t.Fatalf("unhide output = %q", out)
	}
	overview = runCommand(t, []string{"--plain"})
	if !strings.Contains(overview, "\nscanned  ") {
		t.Fatalf("unhidden scanned project missing:\n%s", overview)
	}
}

func TestHiddenFlagListsHiddenProjects(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "visible"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "visible", "go.mod"), []byte("module visible"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "hidden", "go.mod"), []byte("module hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)
	runCommand(t, []string{"hide", "hidden"})

	out := runCommand(t, []string{"--hidden", "--plain"})
	if !strings.Contains(out, "\nhidden  ") || strings.Contains(out, "\nvisible  ") {
		t.Fatalf("hidden overview = %q", out)
	}
	jsonOut := runCommand(t, []string{"--hidden", "--json"})
	if !strings.Contains(jsonOut, `"name": "hidden"`) || strings.Contains(jsonOut, `"name": "visible"`) {
		t.Fatalf("hidden json = %q", jsonOut)
	}
}

func TestScanCommandIsRemoved(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "app", "go.mod"), []byte("module app"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)

	_, err := executeCommand([]string{"scan"})
	if err == nil {
		t.Fatal("expected scan command error")
	}
}

func TestSetUnsetShowCommands(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	runCommand(t, []string{"add", project})

	out := runCommand(t, []string{"set", "custom", "--status", "active", "--note", "fix flow"})
	for _, want := range []string{"Updated custom.", "Status  active", "Note    fix flow"} {
		if !strings.Contains(out, want) {
			t.Fatalf("set output missing %q: %q", want, out)
		}
	}
	show := runCommand(t, []string{"show", "custom"})
	if !bytes.Contains([]byte(show), []byte("Status    active")) || !bytes.Contains([]byte(show), []byte("Note      fix flow")) {
		t.Fatalf("show output = %q", show)
	}

	out = runCommand(t, []string{"unset", "custom", "--note"})
	if !strings.Contains(out, "Updated custom.") || !strings.Contains(out, "Note cleared.") {
		t.Fatalf("unset note output = %q", out)
	}
	show = runCommand(t, []string{"show", "custom"})
	if bytes.Contains([]byte(show), []byte("fix flow")) {
		t.Fatalf("note was not unset: %q", show)
	}

	out = runCommand(t, []string{"unset", "custom", "--status"})
	if !strings.Contains(out, "Updated custom.") || !strings.Contains(out, "Status cleared.") {
		t.Fatalf("unset status output = %q", out)
	}
	show = runCommand(t, []string{"show", "custom"})
	if bytes.Contains([]byte(show), []byte("active")) {
		t.Fatalf("status was not unset: %q", show)
	}
}

func TestSetRequiresStatusOrNote(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	runCommand(t, []string{"add", project})

	_, err := executeCommand([]string{"set", "custom"})
	if err == nil || !strings.Contains(err.Error(), "pass --status or --note") {
		t.Fatalf("set without fields error = %v", err)
	}
	if out, _ := executeCommand([]string{"set", "custom"}); strings.Contains(out, "Usage:") {
		t.Fatalf("set without fields printed usage:\n%s", out)
	}
}

func TestInvalidSortShowsClearErrorWithoutUsage(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	writePackage(t, filepath.Join(root, "app"), `{}`)
	configForTest(t, root)

	out, err := executeCommand([]string{"--sort", "recent"})
	if err == nil || err.Error() != `invalid sort "recent": expected activity, name, or status` {
		t.Fatalf("invalid sort error = %v", err)
	}
	if strings.Contains(out, "Usage:") {
		t.Fatalf("invalid sort printed usage:\n%s", out)
	}

	out, err = executeCommand([]string{"--sort", "name:up"})
	if err == nil || err.Error() != `invalid sort "name:up": expected direction asc or desc` {
		t.Fatalf("invalid sort direction error = %v", err)
	}
	if strings.Contains(out, "Usage:") {
		t.Fatalf("invalid sort direction printed usage:\n%s", out)
	}
}

func TestUnsetRequiresStatusOrNote(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	runCommand(t, []string{"add", project})

	_, err := executeCommand([]string{"unset", "custom"})
	if err == nil || !strings.Contains(err.Error(), "pass --status or --note") {
		t.Fatalf("unset without fields error = %v", err)
	}
}

func TestSetAllowsFreeFormStatus(t *testing.T) {
	home := t.TempDir()
	project := filepath.Join(t.TempDir(), "custom")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, "go.mod"), []byte("module custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	runCommand(t, []string{"add", project})

	out := runCommand(t, []string{"set", "custom", "--status", "needs review"})
	if !strings.Contains(out, "Status  needs review") {
		t.Fatalf("set output = %q", out)
	}
	show := runCommand(t, []string{"show", "custom"})
	if !strings.Contains(show, "Status    needs review") {
		t.Fatalf("show output = %q", show)
	}
}

func TestSetCanTargetScannedProjectByName(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "scanned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scanned", "go.mod"), []byte("module scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)

	runCommand(t, []string{"set", "scanned", "--status", "active"})
	show := runCommand(t, []string{"show", "scanned"})
	if !strings.HasPrefix(show, "scanned\n") {
		t.Fatalf("show output missing title = %q", show)
	}
	if !bytes.Contains([]byte(show), []byte("Path      ")) {
		t.Fatalf("show output missing path = %q", show)
	}
	if !bytes.Contains([]byte(show), []byte("Status    active")) {
		t.Fatalf("show output = %q", show)
	}
	if !bytes.Contains([]byte(show), []byte("Stack     Go")) {
		t.Fatalf("show output missing stack detail = %q", show)
	}
	mustAppearInOrder(t, show, []string{
		"Path",
		"Stack",
		"Manager",
		"Activity",
		"Status",
	})
	for _, unwanted := range []string{
		"State",
		"StackRaw",
		"Managers",
		"ActivityDisplay",
		"LastCommitAge",
		"LastCommitAt",
		"LastCommitMessage",
		"Last commit",
		"Unpushed",
		"Dirty",
		"Git",
		"NoteSource",
		"Manual",
		"Hidden",
	} {
		if bytes.Contains([]byte(show), []byte(unwanted)) {
			t.Fatalf("show output contains noisy field %q = %q", unwanted, show)
		}
	}
	if !bytes.Contains([]byte(show), []byte("Activity  —")) {
		t.Fatalf("show output missing activity detail = %q", show)
	}
}

func TestShowJSONOutputsSingleProject(t *testing.T) {
	home := t.TempDir()
	root := t.TempDir()
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "scanned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scanned", "go.mod"), []byte("module scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)
	runCommand(t, []string{"set", "scanned", "--status", "active", "--note", "working"})

	out := runCommand(t, []string{"show", "scanned", "--json"})
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		t.Fatalf("show json should be one object: %q", out)
	}
	for _, want := range []string{`"name": "scanned"`, `"stack":`, `"Go"`, `"tags":`, `"active"`, `"status": "active"`, `"note": "working"`, `"activity":`} {
		if !strings.Contains(out, want) {
			t.Fatalf("show json missing %q: %s", want, out)
		}
	}
	for _, unwanted := range []string{"stack_display", "custom", "hidden", "last_commit_age"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("show json contains internal field %q: %s", unwanted, out)
		}
	}
}

func TestRootArgShowsSingleProject(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "dev")
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "scanned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scanned", "go.mod"), []byte("module scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)
	runCommand(t, []string{"set", "scanned", "--status", "active", "--note", "working"})

	out := runCommand(t, []string{"scanned"})
	if !strings.HasPrefix(out, "scanned\n") {
		t.Fatalf("root project output missing title = %q", out)
	}
	for _, want := range []string{"Stack     Go", "Manager   go modules", "Status    active", "Note      working"} {
		if !strings.Contains(out, want) {
			t.Fatalf("root project output missing %q: %s", want, out)
		}
	}
	if !strings.Contains(out, "Path      ~/dev/scanned") {
		t.Fatalf("root project output should shorten home path: %s", out)
	}
}

func TestRootArgShowsSingleProjectJSON(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "dev")
	t.Setenv("HOME", home)
	if err := os.MkdirAll(filepath.Join(root, "scanned"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "scanned", "go.mod"), []byte("module scanned"), 0o644); err != nil {
		t.Fatal(err)
	}
	configForTest(t, root)

	out := runCommand(t, []string{"--json", "scanned"})
	trimmed := strings.TrimSpace(out)
	if !strings.HasPrefix(trimmed, "{") || !strings.HasSuffix(trimmed, "}") {
		t.Fatalf("root project json should be one object: %q", out)
	}
	for _, want := range []string{`"name": "scanned"`, `"stack":`, `"Go"`, `"tags":`, `"activity":`} {
		if !strings.Contains(out, want) {
			t.Fatalf("root project json missing %q: %s", want, out)
		}
	}
	if !strings.Contains(out, `"managers":`) || !strings.Contains(out, `"go modules"`) {
		t.Fatalf("root project json missing managers: %s", out)
	}
	if !strings.Contains(out, `"path": "`+filepath.Join(root, "scanned")+`"`) {
		t.Fatalf("root project json should keep absolute path: %s", out)
	}
	if strings.Contains(out, `"path": "~`) {
		t.Fatalf("root project json should not shorten path: %s", out)
	}
}

func TestDetailActivityDoesNotAppendAgoToNow(t *testing.T) {
	got := detailActivity(project.Project{
		Activity: format.ActivityInfo{
			LastCommitAge: "now",
			HasGit:        true,
			HasCommits:    true,
		},
	})
	if got != "now" {
		t.Fatalf("detailActivity() = %q, want now", got)
	}
}

func runCommand(t *testing.T, args []string) string {
	t.Helper()
	out, err := executeCommand(args)
	if err != nil {
		t.Fatalf("Execute(%v) error = %v", args, err)
	}
	return out
}

func executeCommand(args []string) (string, error) {
	cmd := NewRootCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func withTerminalRouting(t *testing.T, interactive bool, runner func(app.Options) error) {
	t.Helper()
	previousTerminal := interactiveTerminal
	previousRunner := runTUI
	interactiveTerminal = func(_ io.Reader, _ io.Writer) bool {
		return interactive
	}
	runTUI = runner
	t.Cleanup(func() {
		interactiveTerminal = previousTerminal
		runTUI = previousRunner
	})
}

func mustAppearInOrder(t *testing.T, text string, values []string) {
	t.Helper()
	offset := 0
	for _, value := range values {
		index := strings.Index(text[offset:], value)
		if index < 0 {
			t.Fatalf("value %q not found after offset %d:\n%s", value, offset, text)
		}
		offset += index + len(value)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func configForTest(t *testing.T, root string) string {
	t.Helper()
	paths, err := config.Paths()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Default()
	cfg.Roots = []string{root}
	if err := config.Write(paths.Config, cfg); err != nil {
		t.Fatal(err)
	}
	return paths.Config
}

func writePackage(t *testing.T, dir, data string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
}
