package description

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"ovw/internal/workspace"
)

func Detect(path string) string {
	roots, err := workspace.CandidateRoots(path)
	if err != nil {
		roots = []string{path}
	}
	for _, root := range roots {
		if description := detectAtRoot(root); description != "" {
			return description
		}
	}
	return ""
}

func detectAtRoot(path string) string {
	for _, detect := range []func(string) string{
		packageJSONDescription,
		cargoDescription,
		pyprojectDescription,
	} {
		if description := detect(path); description != "" {
			return description
		}
	}
	return ""
}

func packageJSONDescription(path string) string {
	data, err := os.ReadFile(filepath.Join(path, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Description
}

func cargoDescription(path string) string {
	data, err := os.ReadFile(filepath.Join(path, "Cargo.toml"))
	if err != nil {
		return ""
	}
	var cargo struct {
		Package struct {
			Description string `toml:"description"`
		} `toml:"package"`
	}
	if err := toml.Unmarshal(data, &cargo); err != nil {
		return ""
	}
	return cargo.Package.Description
}

func pyprojectDescription(path string) string {
	data, err := os.ReadFile(filepath.Join(path, "pyproject.toml"))
	if err != nil {
		return ""
	}
	var pyproject struct {
		Project struct {
			Description string `toml:"description"`
		} `toml:"project"`
	}
	if err := toml.Unmarshal(data, &pyproject); err != nil {
		return ""
	}
	return pyproject.Project.Description
}
