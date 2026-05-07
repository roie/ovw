package app

import (
	"io"
	"time"

	"ovw/internal/cache"
	"ovw/internal/config"
	projectdescription "ovw/internal/description"
	"ovw/internal/filter"
	ovwformat "ovw/internal/format"
	"ovw/internal/gitactivity"
	"ovw/internal/metadata"
	"ovw/internal/project"
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

func Run(opts Options) error {
	start := time.Now()
	if opts.Out == nil {
		opts.Out = io.Discard
	}
	paths, err := config.Paths()
	if err != nil {
		return err
	}
	cfg, _, err := config.Ensure(paths.Config, opts.Cwd, opts.In, firstRunWriter(opts))
	if err != nil {
		return err
	}
	meta, err := metadata.Load(paths.Metadata)
	if err != nil {
		return err
	}
	cacheStore, err := cache.Load(paths.Cache)
	if err != nil {
		return err
	}
	var scanned []scanner.Project
	if opts.Hidden {
		scanned, err = scanner.ScanAll(cfg, meta)
	} else {
		scanned, err = scanner.Scan(cfg, meta)
	}
	if err != nil {
		return err
	}
	projects := make([]project.Project, 0, len(scanned))
	now := time.Now()
	for _, scannedProject := range scanned {
		enriched, cacheProject := Enrich(scannedProject, cfg, cacheStore, now)
		projects = append(projects, enriched)
		cacheStore.Projects[enriched.Path] = cacheProject
	}
	if cfg.Cache.Enabled {
		if err := cache.Write(paths.Cache, cacheStore); err != nil {
			return err
		}
	}
	filtered, err := filter.Apply(projects, filter.Options{
		Status:   opts.Status,
		Dirty:    opts.Dirty,
		Stale:    opts.Stale,
		Untagged: opts.Untagged,
		Hidden:   opts.Hidden,
	}, cfg, now)
	if err != nil {
		return err
	}
	filtered = filter.Sort(filtered, opts.Sort, cfg)
	if opts.JSON {
		return render.JSON(opts.Out, filtered)
	}
	return render.Table(opts.Out, filtered, cfg, time.Since(start))
}

func Enrich(scanned scanner.Project, cfg config.Config, cacheStore cache.Store, now time.Time) (project.Project, cache.Project) {
	stackResult, _ := stack.Detect(scanned.Path, cfg.Stack)
	gitInfo := gitactivity.Detect(scanned.Path)
	cached := cacheStore.Projects[scanned.Path]
	description := cached.Description
	if detectedDescription := projectdescription.Detect(scanned.Path); detectedDescription != "" {
		description = detectedDescription
	}
	activity := ovwformat.Activity(gitInfo, cfg, now)
	note := ovwformat.Note(scanned.Note, description, gitInfo, cfg)
	tags := ovwformat.Tags(activity, scanned.Status, cfg, now)
	return project.Project{
			Name:         scanned.Name,
			Path:         scanned.Path,
			Stack:        stackResult.Labels,
			StackDisplay: stackResult.Display,
			Activity:     activity,
			Tags:         tags,
			Status:       scanned.Status,
			Note:         note,
			Manual:       scanned.Manual,
			Hidden:       scanned.Hidden,
			Description:  description,
		}, cache.Project{
			Stack:             stackResult.Labels,
			StackDisplay:      stackResult.Display,
			Description:       description,
			LastCommitAt:      activity.LastCommitAt,
			LastCommitMessage: activity.LastCommitMessage,
		}
}

func firstRunWriter(opts Options) io.Writer {
	if opts.JSON {
		return io.Discard
	}
	return opts.Out
}
