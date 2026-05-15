package recentfiles

import (
	"time"

	"ovw/internal/projectfiles"
)

type File struct {
	Path       string
	ModifiedAt time.Time
}

func Detect(root string, ignoreDirs []string, limit int) ([]File, error) {
	files, err := projectfiles.Recent(root, projectfiles.Options{IgnoreDirs: ignoreDirs}, limit)
	if err != nil {
		return nil, err
	}
	out := make([]File, 0, len(files))
	for _, file := range files {
		out = append(out, File{Path: file.Path, ModifiedAt: file.ModifiedAt})
	}
	return out, nil
}
