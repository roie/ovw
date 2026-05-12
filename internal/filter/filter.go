package filter

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"ovw/internal/config"
	"ovw/internal/project"
)

type Options struct {
	Status   string
	Path     string
	Dirty    bool
	Stale    bool
	Untagged bool
	Hidden   bool
	Sort     string
}

type SortSpec struct {
	By  string
	Dir string
}

func Apply(projects []project.Project, opts Options, cfg config.Config, now time.Time) ([]project.Project, error) {
	out := []project.Project{}
	pathQuery := strings.ToLower(strings.TrimSpace(opts.Path))
	for _, p := range projects {
		if opts.Hidden && !p.Hidden {
			continue
		}
		if pathQuery != "" && !projectPathMatches(p.Path, pathQuery) {
			continue
		}
		if opts.Status != "" && p.Status.Value != opts.Status {
			continue
		}
		if opts.Dirty && !p.Activity.Dirty {
			continue
		}
		if opts.Stale && !isStale(p, cfg.StaleDays, now) {
			continue
		}
		if opts.Untagged && p.Status.Value != "" {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func projectPathMatches(path, query string) bool {
	path = strings.ToLower(filepath.ToSlash(path))
	query = strings.ToLower(filepath.ToSlash(strings.TrimSpace(query)))
	if query == "" {
		return true
	}
	if strings.Contains(query, "/") {
		return strings.Contains(path, query)
	}
	return strings.Contains(strings.ToLower(filepath.ToSlash(filepath.Dir(path))), query)
}

func Sort(projects []project.Project, mode string, cfg config.Config) []project.Project {
	spec, err := ParseSort(mode, cfg)
	if err != nil {
		spec = SortSpec{By: "activity", Dir: "desc"}
	}
	out := append([]project.Project(nil), projects...)
	desc := spec.Dir == "desc"
	sort.SliceStable(out, func(i, j int) bool {
		switch spec.By {
		case "name":
			if desc {
				return out[i].Name > out[j].Name
			}
			return out[i].Name < out[j].Name
		case "status":
			if desc {
				return out[i].Status.Value > out[j].Status.Value
			}
			return out[i].Status.Value < out[j].Status.Value
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

func ValidateSort(mode string) error {
	if _, _, err := parseSortValue(mode); err != nil {
		return err
	}
	return nil
}

func ValidateSortDir(dir string) error {
	if dir == "" || dir == "asc" || dir == "desc" {
		return nil
	}
	return fmt.Errorf("invalid sort_dir %q: expected asc or desc", dir)
}

func ParseSort(mode string, cfg config.Config) (SortSpec, error) {
	by, dir, err := parseSortValue(mode)
	if err != nil {
		return SortSpec{}, err
	}
	if by == "" {
		by = cfg.SortBy
	}
	if dir == "" {
		dir = cfg.SortDir
	}
	if err := ValidateSort(by); err != nil {
		return SortSpec{}, err
	}
	if err := ValidateSortDir(dir); err != nil {
		return SortSpec{}, err
	}
	return SortSpec{By: by, Dir: dir}, nil
}

func FormatSort(by, dir string) string {
	if dir == "" {
		return by
	}
	return by + ":" + dir
}

func parseSortValue(mode string) (string, string, error) {
	if mode == "" {
		return "", "", nil
	}
	parts := strings.Split(mode, ":")
	if len(parts) > 2 {
		return "", "", fmt.Errorf("invalid sort %q: expected activity, name, or status", mode)
	}
	by := parts[0]
	dir := ""
	if len(parts) == 2 {
		dir = parts[1]
		if dir == "" {
			return "", "", fmt.Errorf("invalid sort %q: expected direction asc or desc", mode)
		}
	}
	switch by {
	case "activity", "name", "status":
	default:
		return "", "", fmt.Errorf("invalid sort %q: expected activity, name, or status", mode)
	}
	if dir != "" && dir != "asc" && dir != "desc" {
		return "", "", fmt.Errorf("invalid sort %q: expected direction asc or desc", mode)
	}
	return by, dir, nil
}

func isStale(p project.Project, staleDays int, now time.Time) bool {
	if !p.Activity.HasCommits || p.Activity.LastCommitAt.IsZero() {
		return false
	}
	return now.Sub(p.Activity.LastCommitAt) > time.Duration(staleDays)*24*time.Hour
}
