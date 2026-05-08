package projectversion

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
		if version := detectAtRoot(root); version != "" {
			return version
		}
	}
	return ""
}

func detectAtRoot(path string) string {
	for _, detect := range []func(string) string{
		packageJSONVersion,
		cargoVersion,
		pyprojectVersion,
		manifestVersion,
	} {
		if version := detect(path); version != "" {
			return version
		}
	}
	return ""
}

func manifestVersion(path string) string {
	data, err := os.ReadFile(filepath.Join(path, "manifest.json"))
	if err != nil {
		return ""
	}
	var manifest struct {
		ManifestVersion int    `json:"manifest_version"`
		Version         string `json:"version"`
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return ""
	}
	if manifest.ManifestVersion == 0 {
		return ""
	}
	return manifest.Version
}

func packageJSONVersion(path string) string {
	data, err := os.ReadFile(filepath.Join(path, "package.json"))
	if err != nil {
		return ""
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return pkg.Version
}

func cargoVersion(path string) string {
	data, err := os.ReadFile(filepath.Join(path, "Cargo.toml"))
	if err != nil {
		return ""
	}
	var cargo struct {
		Package struct {
			Version string `toml:"version"`
		} `toml:"package"`
	}
	if err := toml.Unmarshal(data, &cargo); err != nil {
		return ""
	}
	return cargo.Package.Version
}

func pyprojectVersion(path string) string {
	data, err := os.ReadFile(filepath.Join(path, "pyproject.toml"))
	if err != nil {
		return ""
	}
	var pyproject struct {
		Project struct {
			Version string `toml:"version"`
		} `toml:"project"`
	}
	if err := toml.Unmarshal(data, &pyproject); err != nil {
		return ""
	}
	return pyproject.Project.Version
}
