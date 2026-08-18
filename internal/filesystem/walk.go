// Package filesystem walks configured media roots and reports the media
// files found there. It never writes, renames, or deletes anything — the
// application only ever needs read access to the media mounts.
package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ErrRootInaccessible is returned (wrapped) when a configured media root
// does not exist or cannot be opened.
var ErrRootInaccessible = errors.New("media root inaccessible")

// Root is one configured media directory to walk, labeled by name (e.g.
// "movies", "tv") so downstream matching knows which Arr to consult.
type Root struct {
	Name string
	Path string
}

// File is a single media file found under a Root.
type File struct {
	Root    string
	RelPath string
	Bytes   int64
	ModTime time.Time
}

// RootFailure records a root that could not be walked at all.
type RootFailure struct {
	Root string
	Err  error
}

// Result is everything WalkRoots observed across every configured root.
type Result struct {
	Files        []File
	RootsScanned []string
	RootsFailed  []RootFailure
	// Errors holds non-fatal problems encountered while walking an
	// otherwise-accessible root, e.g. a subdirectory that could not be
	// read due to permissions.
	Errors []error
	// Complete is true only if every root was fully walked with no
	// failures, no errors, and no cancellation. Anything less should be
	// treated as forensic evidence, not a trustworthy full inventory.
	Complete bool
}

// WalkRoots walks every configured root and collects the media files found.
// An inaccessible root does not stop the walk of other roots; it is
// recorded in Result.RootsFailed and reflected in the returned error.
// Context cancellation aborts the walk immediately and is returned as-is.
func WalkRoots(ctx context.Context, roots []Root) (Result, error) {
	result := Result{Complete: true}

	for _, root := range roots {
		select {
		case <-ctx.Done():
			result.Complete = false
			return result, ctx.Err()
		default:
		}

		info, statErr := os.Stat(root.Path)
		if statErr != nil || !info.IsDir() {
			result.RootsFailed = append(result.RootsFailed, RootFailure{
				Root: root.Name,
				Err:  fmt.Errorf("%s: %w: %v", root.Path, ErrRootInaccessible, statErr),
			})
			result.Complete = false
			continue
		}

		files, walkErrs, err := walkOneRoot(ctx, root)
		if err != nil {
			result.Complete = false
			return result, err
		}

		result.Files = append(result.Files, files...)
		result.RootsScanned = append(result.RootsScanned, root.Name)
		if len(walkErrs) > 0 {
			result.Errors = append(result.Errors, walkErrs...)
			result.Complete = false
		}
	}

	sort.Slice(result.Files, func(i, j int) bool {
		if result.Files[i].Root != result.Files[j].Root {
			return result.Files[i].Root < result.Files[j].Root
		}
		return result.Files[i].RelPath < result.Files[j].RelPath
	})

	if len(result.RootsFailed) > 0 {
		errs := make([]error, len(result.RootsFailed))
		for i, rf := range result.RootsFailed {
			errs[i] = rf.Err
		}
		return result, errors.Join(errs...)
	}
	return result, nil
}

// walkOneRoot walks a single, already-verified-accessible root. The third
// return value is non-nil only for context cancellation; every other
// problem (permission denied on a subdirectory, a stat failure on one
// file) is accumulated into the second return value so the walk of
// siblings continues.
func walkOneRoot(ctx context.Context, root Root) ([]File, []error, error) {
	var files []File
	var errs []error

	walkErr := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			return nil
		}

		name := d.Name()

		if d.IsDir() {
			if path != root.Path && shouldSkipDir(name) {
				return filepath.SkipDir
			}
			return nil
		}

		if d.Type()&fs.ModeSymlink != 0 {
			target, statErr := os.Stat(path)
			if statErr != nil {
				errs = append(errs, fmt.Errorf("%s: %w", path, statErr))
				return nil
			}
			if target.IsDir() {
				// Do not follow symlinked directories: avoids symlink loops.
				return nil
			}
			if !isMediaExt(name) {
				return nil
			}
			files = append(files, newFile(root, path, target.Size(), target.ModTime()))
			return nil
		}

		if strings.HasPrefix(name, ".") {
			return nil
		}
		if !isMediaExt(name) {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, infoErr))
			return nil
		}
		files = append(files, newFile(root, path, info.Size(), info.ModTime()))
		return nil
	})

	if walkErr != nil {
		return files, errs, walkErr
	}
	return files, errs, nil
}

func newFile(root Root, path string, size int64, modTime time.Time) File {
	rel, err := filepath.Rel(root.Path, path)
	if err != nil {
		rel = path
	}
	return File{
		Root:    root.Name,
		RelPath: filepath.ToSlash(rel),
		Bytes:   size,
		ModTime: modTime,
	}
}

func shouldSkipDir(name string) bool {
	if strings.HasPrefix(name, ".") {
		return true
	}
	switch name {
	case "@eaDir", "lost+found":
		return true
	}
	return false
}
