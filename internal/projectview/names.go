package projectview

import (
	"path/filepath"

	"ovw/internal/project"
)

func DisambiguatedNames(projects []project.Project) map[string]string {
	counts := map[string]int{}
	for _, project := range projects {
		counts[project.Name]++
	}
	names := map[string]string{}
	for index, project := range projects {
		key := ProjectKey(project, index)
		if counts[project.Name] < 2 {
			names[key] = project.Name
			continue
		}
		parent := filepath.Base(filepath.Dir(project.Path))
		if parent == "." || parent == string(filepath.Separator) || parent == "" {
			names[key] = project.Name
			continue
		}
		names[key] = filepath.ToSlash(filepath.Join(parent, project.Name))
	}
	return names
}

func ProjectKey(project project.Project, index int) string {
	if project.Path != "" {
		return project.Path
	}
	if index < 0 {
		return project.Name
	}
	return project.Name + "\x00" + string(rune(index))
}
