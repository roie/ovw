package project

import (
	"time"

	"ovw/internal/format"
)

type Project struct {
	Name         string              `json:"name"`
	Path         string              `json:"path"`
	Stack        []string            `json:"stack"`
	StackDisplay string              `json:"stack_display"`
	Managers     []string            `json:"managers"`
	Scripts      []string            `json:"scripts"`
	Version      string              `json:"version"`
	Activity     format.ActivityInfo `json:"activity"`
	Status       format.StatusInfo   `json:"status"`
	Note         format.NoteInfo     `json:"note"`
	Hidden       bool                `json:"hidden"`
	Description  string              `json:"-"`
}

func (p Project) LastCommitAt() time.Time {
	return p.Activity.LastCommitAt
}
