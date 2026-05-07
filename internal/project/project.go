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
	Activity     format.ActivityInfo `json:"activity"`
	Tags         []string            `json:"tags"`
	Status       string              `json:"status"`
	Note         format.NoteInfo     `json:"note"`
	Manual       bool                `json:"manual"`
	Hidden       bool                `json:"hidden"`
	Description  string              `json:"-"`
}

func (p Project) LastCommitAt() time.Time {
	return p.Activity.LastCommitAt
}
