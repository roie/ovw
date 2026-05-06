package cache

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Store struct {
	Projects map[string]Project `json:"projects"`
}

type Project struct {
	Stack             []string  `json:"stack,omitempty"`
	StackDisplay      string    `json:"stack_display,omitempty"`
	Description       string    `json:"description,omitempty"`
	LastCommitAt      time.Time `json:"last_commit_at,omitempty"`
	LastCommitMessage string    `json:"last_commit_message,omitempty"`
}

func New() Store {
	return Store{Projects: map[string]Project{}}
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
		return New(), nil
	}
	if store.Projects == nil {
		store.Projects = map[string]Project{}
	}
	return store, nil
}

func Write(path string, store Store) error {
	if store.Projects == nil {
		store.Projects = map[string]Project{}
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

func Clear(path string) error {
	err := os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
