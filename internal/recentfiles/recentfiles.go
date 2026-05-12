package recentfiles

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type File struct {
	Path       string
	ModifiedAt time.Time
}

func Detect(root string, ignoreDirs []string, limit int) ([]File, error) {
	if limit <= 0 {
		return nil, nil
	}
	ignored := map[string]bool{".git": true}
	for _, dir := range ignoreDirs {
		ignored[dir] = true
	}
	var files []File
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() {
			if path != root && (ignored[entry.Name()] || strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, File{
			Path:       filepath.ToSlash(rel),
			ModifiedAt: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].ModifiedAt.Equal(files[j].ModifiedAt) {
			return files[i].Path < files[j].Path
		}
		return files[i].ModifiedAt.After(files[j].ModifiedAt)
	})
	if len(files) > limit {
		files = files[:limit]
	}
	return files, nil
}
