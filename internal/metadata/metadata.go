package metadata

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

type Store struct {
	Projects map[string]Entry `json:"projects"`
}

// Entry is user-owned project metadata. Generated project data such as stack,
// managers, Git activity, descriptions, and recent commits must stay live-only.
type Entry struct {
	Status string `json:"status,omitempty"`
	Note   string `json:"note,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
	Pinned bool   `json:"pinned,omitempty"`
}

func New() Store {
	return Store{Projects: map[string]Entry{}}
}

func Load(path string) (Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return New(), nil
		}
		return Store{}, err
	}
	store := New()
	if err := json.Unmarshal(data, &store); err != nil {
		return Store{}, err
	}
	if store.Projects == nil {
		store.Projects = map[string]Entry{}
	}
	return store, nil
}

func Write(path string, store Store) error {
	if store.Projects == nil {
		store.Projects = map[string]Entry{}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func (s *Store) Set(path string, entry Entry) (string, Entry, error) {
	if s.Projects == nil {
		s.Projects = map[string]Entry{}
	}
	canonical, err := CanonicalPath(path)
	if err != nil {
		return "", Entry{}, err
	}
	current := s.Projects[canonical]
	if entry.Status != "" {
		current.Status = entry.Status
	}
	if entry.Note != "" {
		current.Note = entry.Note
	}
	if entry.Hidden {
		current.Hidden = true
	}
	if entry.Pinned {
		current.Pinned = true
	}
	s.Projects[canonical] = current
	return canonical, current, nil
}

func CanonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	eval, err := filepath.EvalSymlinks(abs)
	if err == nil {
		return eval, nil
	}
	return filepath.Clean(abs), nil
}
