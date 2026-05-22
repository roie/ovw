package columns

import "strings"

var ids = []string{"name", "path", "stack", "manager", "scripts", "version", "ports", "branch", "updated", "activity", "status", "note"}

const FieldPrefix = "field:"

func IDs() []string {
	return append([]string(nil), ids...)
}

func Valid(id string) bool {
	if IsField(id) {
		return true
	}
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func IsField(id string) bool {
	return strings.HasPrefix(id, FieldPrefix) && strings.TrimPrefix(id, FieldPrefix) != ""
}

func FieldID(id string) string {
	if !strings.HasPrefix(id, FieldPrefix) {
		return ""
	}
	return strings.TrimPrefix(id, FieldPrefix)
}

func FieldColumn(id string) string {
	return FieldPrefix + id
}

func Label(id string) string {
	if id == "" {
		return ""
	}
	if fieldID := FieldID(id); fieldID != "" {
		id = fieldID
	}
	return strings.ToUpper(id[:1]) + id[1:]
}

func OptionsString() string {
	if len(ids) == 0 {
		return ""
	}
	if len(ids) == 1 {
		return ids[0]
	}
	return strings.Join(ids[:len(ids)-1], ", ") + ", " + ids[len(ids)-1] + ", or field:<id>"
}
