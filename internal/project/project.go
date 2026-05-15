package project

import (
	"time"

	"ovw/internal/format"
)

type Project struct {
	Name          string              `json:"name"`
	Path          string              `json:"path"`
	Stack         []string            `json:"stack"`
	StackDisplay  string              `json:"stack_display"`
	Managers      []string            `json:"managers"`
	Scripts       []string            `json:"scripts"`
	CustomScripts map[string]string   `json:"-"`
	Version       string              `json:"version"`
	Ports         []int               `json:"ports"`
	Activity      format.ActivityInfo `json:"activity"`
	Status        format.StatusInfo   `json:"status"`
	Note          format.NoteInfo     `json:"note"`
	RecentFiles   []format.RecentFile `json:"-"`
	Hidden        bool                `json:"hidden"`
	Pinned        bool                `json:"pinned"`
	Description   string              `json:"-"`
}

func (p Project) LastCommitAt() time.Time {
	return p.Activity.LastCommitAt
}
