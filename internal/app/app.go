package app

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"ovw/internal/config"
	projectdescription "ovw/internal/description"
	"ovw/internal/filter"
	ovwformat "ovw/internal/format"
	"ovw/internal/gitactivity"
	"ovw/internal/manager"
	"ovw/internal/metadata"
	"ovw/internal/project"
	projectversion "ovw/internal/projectversion"
	"ovw/internal/render"
	"ovw/internal/scanner"
	"ovw/internal/stack"
)

type Options struct {
	Plain    bool
	JSON     bool
	Status   string
	Dirty    bool
	Stale    bool
	Untagged bool
	Hidden   bool
	Sort     string
	Cwd      string
	In       io.Reader
	Out      io.Writer
}

type OverviewResult struct {
	Paths    config.FilePaths
	Config   config.Config
	Projects []project.Project
	Elapsed  time.Duration
}

type State struct {
	Paths  config.FilePaths
	Config config.Config
	Store  metadata.Store
}

type MetadataUpdate struct {
	Status *string
	Note   *string
}

type MetadataUpdateResult struct {
	Path  string
	Entry metadata.Entry
}

func Run(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	overview, err := LoadOverview(opts)
	if err != nil {
		return err
	}
	if opts.JSON {
		return render.JSON(opts.Out, overview.Projects)
	}
	return render.Table(opts.Out, overview.Projects, overview.Config, overview.Elapsed)
}

func LoadOverview(opts Options) (OverviewResult, error) {
	start := time.Now()
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	paths, cfg, err := EnsureConfig(opts)
	if err != nil {
		return OverviewResult{}, err
	}
	meta, err := metadata.Load(paths.Metadata)
	if err != nil {
		return OverviewResult{}, err
	}
	var scanned []scanner.Project
	if opts.Hidden {
		scanned, err = scanner.ScanAll(cfg, meta)
	} else {
		scanned, err = scanner.Scan(cfg, meta)
	}
	if err != nil {
		return OverviewResult{}, err
	}
	projects := make([]project.Project, 0, len(scanned))
	now := time.Now()
	for _, scannedProject := range scanned {
		enriched := Enrich(scannedProject, cfg, now)
		projects = append(projects, enriched)
	}
	filtered, err := filter.Apply(projects, filter.Options{
		Status:   opts.Status,
		Dirty:    opts.Dirty,
		Stale:    opts.Stale,
		Untagged: opts.Untagged,
		Hidden:   opts.Hidden,
	}, cfg, now)
	if err != nil {
		return OverviewResult{}, err
	}
	filtered = filter.Sort(filtered, opts.Sort, cfg)
	return OverviewResult{
		Paths:    paths,
		Config:   cfg,
		Projects: filtered,
		Elapsed:  time.Since(start),
	}, nil
}

func EnsureConfig(opts Options) (config.FilePaths, config.Config, error) {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	paths, err := config.Paths()
	if err != nil {
		return config.FilePaths{}, config.Config{}, err
	}
	cfg, _, err := config.Ensure(paths.Config, opts.Cwd, opts.In, firstRunWriter(opts))
	if err != nil {
		return config.FilePaths{}, config.Config{}, err
	}
	return paths, cfg, nil
}

func LoadState() (State, error) {
	paths, err := config.Paths()
	if err != nil {
		return State{}, err
	}
	cfg, err := config.Load(paths.Config)
	if err != nil {
		if os.IsNotExist(err) {
			cfg = config.Default()
		} else {
			return State{}, err
		}
	}
	store, err := metadata.Load(paths.Metadata)
	if err != nil {
		return State{}, err
	}
	return State{Paths: paths, Config: cfg, Store: store}, nil
}

func ResolveProject(target string, cfg config.Config, store metadata.Store) (string, error) {
	path, err := resolveKnownProject(target, store)
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
		return "", &AmbiguousProjectError{Target: target}
	}
	return "", err
}

type AmbiguousProjectError struct {
	Target string
}

func (err *AmbiguousProjectError) Error() string {
	return "Project " + quoteProject(err.Target) + " is ambiguous; use full path"
}

func ProjectFromPath(path string, cfg config.Config, store metadata.Store, now time.Time) project.Project {
	entry := store.Projects[path]
	return Enrich(scanner.Project{
		Name:   filepath.Base(path),
		Path:   path,
		Manual: entry.Manual,
		Hidden: entry.Hidden,
		Status: entry.Status,
		Note:   entry.Note,
	}, cfg, now)
}

func UpdateProjectMetadata(target string, update MetadataUpdate) (MetadataUpdateResult, error) {
	state, err := LoadState()
	if err != nil {
		return MetadataUpdateResult{}, err
	}
	path, err := ResolveProject(target, state.Config, state.Store)
	if err != nil {
		return MetadataUpdateResult{}, err
	}
	entry := state.Store.Projects[path]
	if update.Status != nil {
		entry.Status = *update.Status
	}
	if update.Note != nil {
		entry.Note = *update.Note
	}
	state.Store.Projects[path] = entry
	if err := metadata.Write(state.Paths.Metadata, state.Store); err != nil {
		return MetadataUpdateResult{}, err
	}
	return MetadataUpdateResult{Path: path, Entry: entry}, nil
}

func SetProjectHidden(target string, hidden bool) (MetadataUpdateResult, error) {
	state, err := LoadState()
	if err != nil {
		return MetadataUpdateResult{}, err
	}
	path, err := ResolveProject(target, state.Config, state.Store)
	if err != nil {
		return MetadataUpdateResult{}, err
	}
	entry := state.Store.Projects[path]
	entry.Hidden = hidden
	state.Store.Projects[path] = entry
	if err := metadata.Write(state.Paths.Metadata, state.Store); err != nil {
		return MetadataUpdateResult{}, err
	}
	return MetadataUpdateResult{Path: path, Entry: entry}, nil
}

func resolveKnownProject(target string, store metadata.Store) (string, error) {
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
		return "", &AmbiguousProjectError{Target: target}
	}
	return "", &ProjectNotFoundError{Target: target}
}

type ProjectNotFoundError struct {
	Target string
}

func (err *ProjectNotFoundError) Error() string {
	return "project not found: " + err.Target
}

func Enrich(scanned scanner.Project, cfg config.Config, now time.Time) project.Project {
	stackResult, _ := stack.Detect(scanned.Path, cfg.Stack)
	managers := manager.Detect(scanned.Path)
	gitInfo := gitactivity.Detect(scanned.Path)
	description := projectdescription.Detect(scanned.Path)
	version := projectversion.Detect(scanned.Path)
	activity := ovwformat.Activity(gitInfo, cfg, now)
	note := ovwformat.Note(scanned.Note, description, gitInfo, cfg)
	status := ovwformat.Status(activity, scanned.Status, cfg, now)
	return project.Project{
		Name:         scanned.Name,
		Path:         scanned.Path,
		Stack:        stackResult.Labels,
		StackDisplay: stackResult.Display,
		Managers:     managers,
		Version:      version,
		Activity:     activity,
		Status:       status,
		Note:         note,
		Manual:       scanned.Manual,
		Hidden:       scanned.Hidden,
		Description:  description,
	}
}

func firstRunWriter(opts Options) io.Writer {
	if opts.JSON {
		return io.Discard
	}
	return opts.Out
}

func quoteProject(value string) string {
	return `"` + value + `"`
}
