package scripts

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"

	"ovw/internal/workspace"
)

func Detect(path string) []string {
	return DetectWithIgnore(path, nil)
}

func DetectWithIgnore(path string, ignoreDirs []string) []string {
	roots, err := workspace.CandidateRootsWithIgnore(path, ignoreDirs)
	if err != nil {
		roots = []string{path}
	}
	seen := map[string]bool{}
	out := []string{}
	for _, root := range roots {
		for _, script := range detectAtRoot(root) {
			if seen[script] {
				continue
			}
			seen[script] = true
			out = append(out, script)
		}
	}
	return out
}

func detectAtRoot(path string) []string {
	scripts := []string{}
	scripts = append(scripts, packageJSONScripts(filepath.Join(path, "package.json"))...)
	scripts = append(scripts, cargoAliases(filepath.Join(path, ".cargo", "config.toml"))...)
	scripts = append(scripts, pyprojectTasks(filepath.Join(path, "pyproject.toml"))...)
	return scripts
}

func packageJSONScripts(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts orderedScripts `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return pkg.Scripts.Names
}

func cargoAliases(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var cfg struct {
		Alias map[string]any `toml:"alias"`
	}
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	return orderedTomlKeys(data, "alias", cfg.Alias)
}

func pyprojectTasks(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pyproject struct {
		Tool struct {
			Poe struct {
				Tasks map[string]any `toml:"tasks"`
			} `toml:"poe"`
			Taskipy struct {
				Tasks map[string]any `toml:"tasks"`
			} `toml:"taskipy"`
		} `toml:"tool"`
	}
	if err := toml.Unmarshal(data, &pyproject); err != nil {
		return nil
	}
	scripts := orderedTomlKeys(data, "tool.poe.tasks", pyproject.Tool.Poe.Tasks)
	scripts = append(scripts, orderedTomlKeys(data, "tool.taskipy.tasks", pyproject.Tool.Taskipy.Tasks)...)
	return scripts
}

func orderedTomlKeys(data []byte, section string, values map[string]any) []string {
	if len(values) == 0 {
		return nil
	}
	found := map[string]bool{}
	scripts := []string{}
	inSection := false
	for _, rawLine := range bytes.Split(data, []byte("\n")) {
		line := bytes.TrimSpace(rawLine)
		if len(line) == 0 || bytes.HasPrefix(line, []byte("#")) {
			continue
		}
		if bytes.HasPrefix(line, []byte("[")) && bytes.HasSuffix(line, []byte("]")) {
			current := string(bytes.Trim(line, "[]"))
			inSection = current == section
			if strings.HasPrefix(current, section+".") {
				name := trimTomlKeyQuotes(strings.TrimPrefix(current, section+"."))
				if _, ok := values[name]; ok && !found[name] {
					found[name] = true
					scripts = append(scripts, name)
				}
			}
			continue
		}
		if !inSection {
			continue
		}
		key, _, ok := bytes.Cut(line, []byte("="))
		if !ok {
			continue
		}
		name := trimTomlKeyQuotes(string(bytes.TrimSpace(key)))
		if _, ok := values[name]; ok && !found[name] {
			found[name] = true
			scripts = append(scripts, name)
		}
	}
	for name := range values {
		if !found[name] {
			scripts = append(scripts, name)
		}
	}
	return scripts
}

func trimTomlKeyQuotes(value string) string {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

type orderedScripts struct {
	Names []string
}

func (scripts *orderedScripts) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return nil
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		name, ok := token.(string)
		if !ok {
			continue
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		scripts.Names = append(scripts.Names, name)
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	return nil
}
