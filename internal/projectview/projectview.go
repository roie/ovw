package projectview

import (
	"fmt"
	"strings"
	"time"

	"ovw/internal/project"
)

type Field struct {
	Label string
	Value string
}

type Options struct {
	Path     func(string) string
	Time     func(time.Time) string
	Activity func(project.Project) string
}

func Fields(project project.Project, opts Options) []Field {
	path := project.Path
	if opts.Path != nil {
		path = opts.Path(project.Path)
	}
	activity := project.Activity.Display
	if opts.Activity != nil {
		activity = opts.Activity(project)
	}

	fields := []Field{
		{Label: "Path", Value: path},
		{Label: "Stack", Value: stackDisplay(project.Stack)},
		{Label: "Manager", Value: managerDisplay(project.Managers)},
		{Label: "Scripts", Value: scriptsDisplay(project.Scripts)},
	}
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, Field{Label: label, Value: value})
		}
	}

	add("Version", project.Version)
	add("Ports", portsDisplay(project.Ports))
	add("Branch", project.Activity.Branch)
	add("Activity", activity)
	if !project.Activity.LastCommitAt.IsZero() {
		updated := project.Activity.LastCommitAt.Format("2006-01-02 15:04")
		if opts.Time != nil {
			updated = opts.Time(project.Activity.LastCommitAt)
		}
		add("Updated", updated)
	}
	add("Status", project.Status.Display)
	add("Note", project.Note.Display)
	return fields
}

func Subtitle(project project.Project) string {
	return project.Description
}

func DetailActivity(project project.Project) string {
	activity := project.Activity
	if !activity.HasGit || !activity.HasCommits {
		return activity.Display
	}
	age := activity.LastCommitAge
	if age != "now" {
		age += " ago"
	}
	parts := []string{age}
	if activity.Unpushed > 0 {
		parts = append(parts, fmt.Sprintf("%d unpushed", activity.Unpushed))
	}
	return strings.Join(parts, " · ")
}

func stackDisplay(labels []string) string {
	if len(labels) == 0 {
		return "—"
	}
	return strings.Join(labels, ", ")
}

func managerDisplay(managers []string) string {
	if len(managers) == 0 {
		return "—"
	}
	return strings.Join(managers, ", ")
}

func scriptsDisplay(scripts []string) string {
	if len(scripts) == 0 {
		return "—"
	}
	return strings.Join(scripts, ", ")
}
