package scripts

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
)

func Detect(path string) []string {
	return packageJSONScripts(filepath.Join(path, "package.json"))
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
