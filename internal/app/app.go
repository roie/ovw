package app

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"ovw/internal/config"
	projectdescription "ovw/internal/description"
	"ovw/internal/filter"
	ovwformat "ovw/internal/format"
	"ovw/internal/gitactivity"
	"ovw/internal/manager"
	"ovw/internal/metadata"
	"ovw/internal/ports"
	"ovw/internal/project"
	projectversion "ovw/internal/projectversion"
	"ovw/internal/render"
	"ovw/internal/scanner"
	"ovw/internal/scripts"
	"ovw/internal/stack"
)

var detectPorts = ports.Detect

const DefaultEnrichmentWorkers = 8

type Options struct {
	Plain    bool
	JSON     bool
	Open     bool
	Status   string
	Path     string
	Dirty    bool
	Stale    bool
	Untagged bool
	Hidden   bool
	Sort     string
	Timing   bool
	Cwd      string
	In       io.Reader
	Out      io.Writer
	Err      io.Writer
}

type OverviewResult struct {
	Paths    config.FilePaths
	Config   config.Config
	Projects []project.Project
	Scanned  []scanner.Project
	Elapsed  time.Duration
	Timing   timingInfo
}

type timingInfo struct {
	Config     time.Duration
	Metadata   time.Duration
	Discover   time.Duration
	Enrich     time.Duration
	Ports      time.Duration
	FilterSort time.Duration
	Total      time.Duration
	PortsRan   bool
	Slow       []projectTiming
}

type projectTiming struct {
	Name     string
	Path     string
	Duration time.Duration
}

type ConfigSetup struct {
	Paths      config.FilePaths
	Exists     bool
	Candidates []string
}

type State struct {
	Paths  config.FilePaths
	Config config.Config
	Store  metadata.Store
}

type MetadataUpdate struct {
	Status *string
	Note   *string
	Pinned *bool
}

type MetadataUpdateResult struct {
	Path  string
	Entry metadata.Entry
}

type AddProjectResult struct {
	Path           string
	AlreadyTracked bool
}

type errEmptyProjectPath struct{}

func (errEmptyProjectPath) Error() string {
	return "select at least one project root"
}

func Run(opts Options) error {
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	if err := ValidateOptions(opts); err != nil {
		return err
	}
	overview, err := LoadOverview(opts)
	if err != nil {
		return err
	}
	if opts.JSON {
		if err := render.JSON(opts.Out, overview.Projects); err != nil {
			return err
		}
		writeTiming(opts, overview.Timing)
		return nil
	}
	if err := render.Table(opts.Out, overview.Projects, overview.Config, overview.Elapsed); err != nil {
		return err
	}
	writeTiming(opts, overview.Timing)
	return nil
}

func LoadOverview(opts Options) (OverviewResult, error) {
	start := time.Now()
	timing := timingInfo{}
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	if err := ValidateOptions(opts); err != nil {
		return OverviewResult{}, err
	}
	phaseStart := time.Now()
	paths, cfg, err := EnsureConfig(opts)
	if err != nil {
		return OverviewResult{}, err
	}
	timing.Config = time.Since(phaseStart)
	if _, err := filter.ParseSort(opts.Sort, cfg); err != nil {
		return OverviewResult{}, err
	}
	phaseStart = time.Now()
	meta, err := metadata.Load(paths.Metadata)
	if err != nil {
		return OverviewResult{}, err
	}
	timing.Metadata = time.Since(phaseStart)
	phaseStart = time.Now()
	var scanned []scanner.Project
	if opts.Hidden {
		scanned, err = scanner.ScanAll(cfg, meta)
	} else {
		scanned, err = scanner.Scan(cfg, meta)
	}
	if err != nil {
		return OverviewResult{}, err
	}
	timing.Discover = time.Since(phaseStart)
	phaseStart = time.Now()
	now := time.Now()
	projects, slow := enrichProjects(scanned, cfg, now, DefaultEnrichmentWorkers)
	timing.Slow = slow
	timing.Enrich = time.Since(phaseStart)
	if shouldDetectPorts(cfg, opts) {
		phaseStart = time.Now()
		attachPorts(projects, detectPorts(projectPaths(projects)))
		timing.Ports = time.Since(phaseStart)
		timing.PortsRan = true
	}
	phaseStart = time.Now()
	filtered, err := filter.Apply(projects, filter.Options{
		Status:   opts.Status,
		Path:     opts.Path,
		Dirty:    opts.Dirty,
		Stale:    opts.Stale,
		Untagged: opts.Untagged,
		Hidden:   opts.Hidden,
	}, cfg, now)
	if err != nil {
		return OverviewResult{}, err
	}
	filtered = filter.Sort(filtered, opts.Sort, cfg)
	timing.FilterSort = time.Since(phaseStart)
	timing.Total = time.Since(start)
	return OverviewResult{
		Paths:    paths,
		Config:   cfg,
		Projects: filtered,
		Scanned:  scanned,
		Elapsed:  timing.Total,
		Timing:   timing,
	}, nil
}

func enrichProjects(scanned []scanner.Project, cfg config.Config, now time.Time, workers int) ([]project.Project, []projectTiming) {
	if len(scanned) == 0 {
		return nil, nil
	}
	if workers <= 0 {
		workers = DefaultEnrichmentWorkers
	}
	if workers > len(scanned) {
		workers = len(scanned)
	}
	type result struct {
		index    int
		project  project.Project
		duration time.Duration
	}
	jobs := make(chan int)
	results := make(chan result, len(scanned))
	detector := gitactivity.NewDetector()
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for index := range jobs {
				scannedProject := scanned[index]
				projectStart := time.Now()
				enriched := EnrichWithGit(scannedProject, cfg, now, detector.Detect(scannedProject.Path))
				results <- result{index: index, project: enriched, duration: time.Since(projectStart)}
			}
		}()
	}
	for index := range scanned {
		jobs <- index
	}
	close(jobs)
	wg.Wait()
	close(results)

	projects := make([]project.Project, len(scanned))
	slow := make([]projectTiming, 0, len(scanned))
	for result := range results {
		projects[result.index] = result.project
		scannedProject := scanned[result.index]
		slow = append(slow, projectTiming{
			Name:     scannedProject.Name,
			Path:     scannedProject.Path,
			Duration: result.duration,
		})
	}
	return projects, slow
}

func DiscoverOverview(opts Options) (OverviewResult, error) {
	start := time.Now()
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	if opts.Err == nil {
		opts.Err = io.Discard
	}
	if err := ValidateOptions(opts); err != nil {
		return OverviewResult{}, err
	}
	paths, cfg, err := EnsureConfig(opts)
	if err != nil {
		return OverviewResult{}, err
	}
	if _, err := filter.ParseSort(opts.Sort, cfg); err != nil {
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
	projects := placeholderProjects(scanned)
	filtered, err := filter.Apply(projects, filter.Options{
		Path:   opts.Path,
		Hidden: opts.Hidden,
	}, cfg, time.Now())
	if err != nil {
		return OverviewResult{}, err
	}
	filtered = filter.Sort(filtered, filter.FormatSort("name", "asc"), cfg)
	return OverviewResult{
		Paths:    paths,
		Config:   cfg,
		Projects: filtered,
		Scanned:  filterScannedByProjects(scanned, filtered),
		Elapsed:  time.Since(start),
	}, nil
}

func placeholderProjects(scanned []scanner.Project) []project.Project {
	projects := make([]project.Project, 0, len(scanned))
	for _, scannedProject := range scanned {
		projects = append(projects, project.Project{
			Name:   scannedProject.Name,
			Path:   scannedProject.Path,
			Hidden: scannedProject.Hidden,
			Pinned: scannedProject.Pinned,
		})
	}
	return projects
}

func filterScannedByProjects(scanned []scanner.Project, projects []project.Project) []scanner.Project {
	visible := map[string]bool{}
	for _, project := range projects {
		visible[project.Path] = true
	}
	out := make([]scanner.Project, 0, len(projects))
	for _, scannedProject := range scanned {
		if visible[scannedProject.Path] {
			out = append(out, scannedProject)
		}
	}
	return out
}

func writeTiming(opts Options, timing timingInfo) {
	if !opts.Timing {
		return
	}
	errOut := opts.Err
	if errOut == nil {
		errOut = io.Discard
	}
	fmt.Fprintf(errOut, "timing: total %s\n", formatDuration(timing.Total))
	fmt.Fprintf(errOut, "timing: config %s\n", formatDuration(timing.Config))
	fmt.Fprintf(errOut, "timing: metadata %s\n", formatDuration(timing.Metadata))
	fmt.Fprintf(errOut, "timing: discover %s\n", formatDuration(timing.Discover))
	fmt.Fprintf(errOut, "timing: enrich %s\n", formatDuration(timing.Enrich))
	if timing.PortsRan {
		fmt.Fprintf(errOut, "timing: ports %s\n", formatDuration(timing.Ports))
	} else {
		fmt.Fprintln(errOut, "timing: ports skipped")
	}
	fmt.Fprintf(errOut, "timing: filter/sort %s\n", formatDuration(timing.FilterSort))
	slow := slowProjects(timing.Slow, 5)
	if len(slow) == 0 {
		return
	}
	fmt.Fprintln(errOut, "timing: slow projects")
	for _, item := range slow {
		fmt.Fprintf(errOut, "timing:   %s %s %s\n", item.Name, formatDuration(item.Duration), item.Path)
	}
}

func slowProjects(items []projectTiming, limit int) []projectTiming {
	sort.SliceStable(items, func(i, j int) bool {
		return items[i].Duration > items[j].Duration
	})
	if len(items) > limit {
		items = items[:limit]
	}
	return items
}

func formatDuration(value time.Duration) string {
	if value > 0 && value < time.Millisecond {
		return "<1ms"
	}
	if value < time.Second {
		return value.Round(time.Millisecond).String()
	}
	return value.Round(100 * time.Millisecond).String()
}

func ValidateOptions(opts Options) error {
	if opts.Plain && opts.JSON {
		return errors.New("choose only one output mode: --plain or --json")
	}
	if opts.Timing && !opts.Plain && !opts.JSON {
		return errors.New("--timing requires --plain or --json")
	}
	return nil
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

func CheckConfig(opts Options) (ConfigSetup, error) {
	paths, err := config.Paths()
	if err != nil {
		return ConfigSetup{}, err
	}
	if _, err := config.Load(paths.Config); err == nil {
		return ConfigSetup{Paths: paths, Exists: true}, nil
	} else if !os.IsNotExist(err) {
		return ConfigSetup{}, err
	}
	candidates, err := config.RootCandidates(opts.Cwd)
	if err != nil {
		return ConfigSetup{}, err
	}
	return ConfigSetup{Paths: paths, Candidates: candidates}, nil
}

func CreateConfig(root string) (config.FilePaths, config.Config, error) {
	return CreateConfigRoots([]string{root})
}

func CreateConfigRoots(roots []string) (config.FilePaths, config.Config, error) {
	paths, err := config.Paths()
	if err != nil {
		return config.FilePaths{}, config.Config{}, err
	}
	if len(roots) == 0 {
		return config.FilePaths{}, config.Config{}, errEmptyProjectPath{}
	}
	for _, root := range roots {
		expanded, err := config.ExpandPath(root)
		if err != nil {
			return config.FilePaths{}, config.Config{}, err
		}
		info, err := os.Stat(expanded)
		if err != nil {
			return config.FilePaths{}, config.Config{}, err
		}
		if !info.IsDir() {
			return config.FilePaths{}, config.Config{}, &NotDirectoryError{Path: root}
		}
	}
	cfg := config.Default()
	cfg.Roots = roots
	if err := config.Validate(cfg); err != nil {
		return config.FilePaths{}, config.Config{}, err
	}
	if err := config.WriteDefault(paths.Config, cfg.Roots); err != nil {
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
	enriched := Enrich(scanner.Project{
		Name:   filepath.Base(path),
		Path:   path,
		Hidden: entry.Hidden,
		Pinned: entry.Pinned,
		Status: entry.Status,
		Note:   entry.Note,
	}, cfg, now)
	projects := []project.Project{enriched}
	attachPorts(projects, detectPorts([]string{path}))
	return projects[0]
}

func shouldDetectPorts(cfg config.Config, _ Options) bool {
	for _, column := range cfg.Columns {
		if column == "ports" {
			return true
		}
	}
	return false
}

func projectPaths(projects []project.Project) []string {
	paths := make([]string, 0, len(projects))
	for _, project := range projects {
		paths = append(paths, project.Path)
	}
	return paths
}

func attachPorts(projects []project.Project, portsByPath map[string][]int) {
	for index := range projects {
		projects[index].Ports = portsByPath[projects[index].Path]
	}
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
	if update.Pinned != nil {
		entry.Pinned = *update.Pinned
	}
	state.Store.Projects[path] = entry
	if err := metadata.Write(state.Paths.Metadata, state.Store); err != nil {
		return MetadataUpdateResult{}, err
	}
	return MetadataUpdateResult{Path: path, Entry: entry}, nil
}

func AddProject(path string) (AddProjectResult, error) {
	projectPath, err := config.ExpandPath(path)
	if err != nil {
		return AddProjectResult{}, err
	}
	info, err := os.Stat(projectPath)
	if err != nil {
		return AddProjectResult{}, err
	}
	if !info.IsDir() {
		return AddProjectResult{}, &NotDirectoryError{Path: path}
	}
	paths, err := config.Paths()
	if err != nil {
		return AddProjectResult{}, err
	}
	cfg, err := config.Load(paths.Config)
	if err != nil && !os.IsNotExist(err) {
		return AddProjectResult{}, err
	}
	if os.IsNotExist(err) {
		cfg = config.Default()
		cfg.Roots = nil
	}
	canonical, err := metadata.CanonicalPath(projectPath)
	if err != nil {
		return AddProjectResult{}, err
	}
	for _, root := range cfg.Roots {
		expanded, err := config.ExpandPath(root)
		if err != nil {
			continue
		}
		existing, err := metadata.CanonicalPath(expanded)
		if err != nil {
			continue
		}
		if existing == canonical {
			return AddProjectResult{Path: canonical, AlreadyTracked: true}, nil
		}
	}
	cfg.Roots = append(cfg.Roots, path)
	if err := config.Validate(cfg); err != nil {
		return AddProjectResult{}, err
	}
	if err := config.WriteDefault(paths.Config, cfg.Roots); err != nil {
		return AddProjectResult{}, err
	}
	return AddProjectResult{Path: canonical}, nil
}

type NotDirectoryError struct {
	Path string
}

func (err *NotDirectoryError) Error() string {
	return err.Path + " is not a directory"
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
	return EnrichWithGit(scanned, cfg, now, gitactivity.Detect(scanned.Path))
}

func EnrichWithGit(scanned scanner.Project, cfg config.Config, now time.Time, gitInfo gitactivity.Info) project.Project {
	stackResult, _ := stack.Detect(scanned.Path, cfg.Stack)
	managers := manager.Detect(scanned.Path)
	detectedScripts := scripts.Detect(scanned.Path)
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
		Scripts:      detectedScripts,
		Version:      version,
		Activity:     activity,
		Status:       status,
		Note:         note,
		Hidden:       scanned.Hidden,
		Pinned:       scanned.Pinned,
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
