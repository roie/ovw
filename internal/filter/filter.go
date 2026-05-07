package filter

import (
	"sort"
	"strings"
	"time"

	"ovw/internal/config"
	"ovw/internal/project"
)

type Options struct {
	Status   string
	Dirty    bool
	Stale    bool
	Untagged bool
	Hidden   bool
	Sort     string
}

func Apply(projects []project.Project, opts Options, cfg config.Config, now time.Time) ([]project.Project, error) {
	out := []project.Project{}
	for _, p := range projects {
		if opts.Hidden && !p.Hidden {
			continue
		}
		if opts.Status != "" && p.Status != opts.Status {
			continue
		}
		if opts.Dirty && !p.Activity.Dirty {
			continue
		}
		if opts.Stale && !isStale(p, cfg.StaleDays, now) {
			continue
		}
		if opts.Untagged && p.Status != "" {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func Sort(projects []project.Project, mode string, cfg config.Config) []project.Project {
	if mode == "" {
		mode = cfg.SortBy
	}
	out := append([]project.Project(nil), projects...)
	desc := strings.EqualFold(cfg.SortDir, "desc")
	sort.SliceStable(out, func(i, j int) bool {
		switch mode {
		case "name":
			if desc {
				return out[i].Name > out[j].Name
			}
			return out[i].Name < out[j].Name
		case "status":
			if desc {
				return out[i].Status > out[j].Status
			}
			return out[i].Status < out[j].Status
		case "activity":
			fallthrough
		default:
			left := out[i].Activity.LastCommitAt
			right := out[j].Activity.LastCommitAt
			if left.Equal(right) {
				return out[i].Name < out[j].Name
			}
			if desc {
				return left.After(right)
			}
			return left.Before(right)
		}
	})
	return out
}

func isStale(p project.Project, staleDays int, now time.Time) bool {
	if !p.Activity.HasCommits || p.Activity.LastCommitAt.IsZero() {
		return false
	}
	return now.Sub(p.Activity.LastCommitAt) > time.Duration(staleDays)*24*time.Hour
}
