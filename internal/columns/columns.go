package columns

import "strings"

var ids = []string{"name", "path", "stack", "manager", "scripts", "version", "ports", "activity", "status", "note"}

func IDs() []string {
	return append([]string(nil), ids...)
}

func Valid(id string) bool {
	for _, candidate := range ids {
		if candidate == id {
			return true
		}
	}
	return false
}

func Label(id string) string {
	if id == "" {
		return ""
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
	return strings.Join(ids[:len(ids)-1], ", ") + ", or " + ids[len(ids)-1]
}
