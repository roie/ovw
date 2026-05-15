package projectfiles

import (
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/boyter/gocodewalker"
)

type Options struct {
	IgnoreDirs []string
}

type File struct {
	Path       string
	ModifiedAt time.Time
}

func Recent(root string, opts Options, limit int) ([]File, error) {
	if limit <= 0 {
		return nil, nil
	}
	files, err := walk(root, opts)
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

func NewestModified(root string, opts Options) (time.Time, bool, error) {
	files, err := Recent(root, opts, 1)
	if err != nil {
		return time.Time{}, false, err
	}
	if len(files) == 0 {
		return time.Time{}, false, nil
	}
	return files[0].ModifiedAt, true, nil
}

func walk(root string, opts Options) ([]File, error) {
	queue := make(chan *gocodewalker.File, 128)
	walker := gocodewalker.NewFileWalker(root, queue)
	walker.IncludeHidden = true
	walker.ExcludeDirectory = append(walker.ExcludeDirectory, ".git")
	walker.ExcludeDirectory = append(walker.ExcludeDirectory, ".cache", ".next", ".nuxt", ".svelte-kit", ".turbo", ".venv", ".vite")
	walker.ExcludeDirectory = append(walker.ExcludeDirectory, opts.IgnoreDirs...)
	walker.SetConcurrency(4)

	var walkErr error
	done := make(chan struct{})
	go func() {
		walkErr = walker.Start()
		close(done)
	}()

	files := []File{}
	for file := range queue {
		info, err := os.Stat(file.Location)
		if err != nil || info.IsDir() {
			continue
		}
		rel, err := filepath.Rel(root, file.Location)
		if err != nil {
			continue
		}
		files = append(files, File{
			Path:       filepath.ToSlash(rel),
			ModifiedAt: info.ModTime(),
		})
	}
	<-done
	if walkErr != nil {
		return nil, walkErr
	}
	return files, nil
}
