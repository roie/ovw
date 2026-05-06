package scanner

import (
	"os"
	"path/filepath"
	"sort"

	"ovw/internal/config"
	"ovw/internal/metadata"
)

type Project struct {
	Name   string
	Path   string
	Manual bool
	Hidden bool
	Status string
	Note   string
}

func Scan(cfg config.Config, meta metadata.Store) ([]Project, error) {
	seen := map[string]Project{}
	for _, root := range cfg.Roots {
		expanded, err := config.ExpandPath(root)
		if err != nil {
			return nil, err
		}
		if info, err := os.Stat(expanded); err != nil || !info.IsDir() {
			continue
		}
		rootDepth := len(splitPath(expanded))
		walkErr := filepath.WalkDir(expanded, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() {
				return nil
			}
			if path != expanded && shouldIgnore(d.Name(), cfg.IgnoreDirs) {
				return filepath.SkipDir
			}
			depth := len(splitPath(path)) - rootDepth
			if cfg.MaxDepth > 0 && depth > cfg.MaxDepth {
				return filepath.SkipDir
			}
			if IsProject(path, cfg.ProjectMarkers) {
				addProject(seen, path, meta)
				if path != expanded && !cfg.ScanNestedProjects {
					return filepath.SkipDir
				}
			}
			return nil
		})
		if walkErr != nil {
			return nil, walkErr
		}
	}
	for path, entry := range meta.Projects {
		if entry.Manual {
			addProject(seen, path, meta)
		}
	}
	projects := make([]Project, 0, len(seen))
	for _, project := range seen {
		if project.Hidden {
			continue
		}
		projects = append(projects, project)
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Path < projects[j].Path
	})
	return projects, nil
}

func IsProject(path string, markers []string) bool {
	for _, marker := range markers {
		if _, err := os.Stat(filepath.Join(path, marker)); err == nil {
			return true
		}
	}
	return false
}

func DetectCurrentRoot(path string, markers []string) bool {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false
	}
	count := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if IsProject(filepath.Join(path, entry.Name()), markers) {
			count++
			if count >= 2 {
				return true
			}
		}
	}
	return false
}

func addProject(seen map[string]Project, path string, meta metadata.Store) {
	canonical, err := metadata.CanonicalPath(path)
	if err != nil {
		return
	}
	entry := meta.Projects[canonical]
	seen[canonical] = Project{
		Name:   filepath.Base(canonical),
		Path:   canonical,
		Manual: entry.Manual,
		Hidden: entry.Hidden,
		Status: entry.Status,
		Note:   entry.Note,
	}
}

func shouldIgnore(name string, ignoreDirs []string) bool {
	for _, ignored := range ignoreDirs {
		if name == ignored {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	clean := filepath.Clean(path)
	parts := []string{}
	for {
		dir, file := filepath.Split(clean)
		if file != "" {
			parts = append(parts, file)
		}
		next := filepath.Clean(dir)
		if next == clean || next == "." || next == string(filepath.Separator) {
			break
		}
		clean = next
	}
	return parts
}
