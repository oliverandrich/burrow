package dev

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// watcher walks a project tree, subscribes to fsnotify events on every
// non-excluded directory, and turns bursts of file events into single
// debounced restart triggers.
type watcher struct {
	cfg     Config
	fsn     *fsnotify.Watcher
	excPath []string // path-shaped excludes (contain a separator)
	excName []string // bare-name excludes (matched anywhere)
}

func newWatcher(cfg Config) (*watcher, error) {
	fsn, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &watcher{cfg: cfg, fsn: fsn}
	w.excName, w.excPath = splitExcludes(cfg.ExcludeDirs)

	if err := w.addRecursive(cfg.ProjectRoot); err != nil {
		_ = fsn.Close()
		return nil, err
	}
	return w, nil
}

// Close releases the underlying fsnotify watcher.
func (w *watcher) Close() error { return w.fsn.Close() }

// Run blocks until ctx is cancelled, invoking trigger once per
// debounce window for any qualifying file event.
func (w *watcher) Run(ctx context.Context, trigger func()) {
	var (
		timer  *time.Timer
		timerC <-chan time.Time
	)
	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return

		case ev, ok := <-w.fsn.Events:
			if !ok {
				return
			}
			// Pick up directories that didn't exist at startup —
			// e.g. a newly-created package — so files written inside
			// them after the mkdir are observed without a restart.
			if ev.Op&fsnotify.Create != 0 {
				w.addIfNewDir(ev.Name)
			}
			if !w.qualifies(ev) {
				continue
			}
			if timer == nil {
				timer = time.NewTimer(w.cfg.Debounce)
				timerC = timer.C
			} else {
				if !timer.Stop() {
					// timer already fired; drain its channel if present.
					select {
					case <-timer.C:
					default:
					}
				}
				timer.Reset(w.cfg.Debounce)
			}

		case <-timerC:
			timer = nil
			timerC = nil
			trigger()

		case _, ok := <-w.fsn.Errors:
			if !ok {
				return
			}
			// fsnotify errors are advisory; we keep watching.
		}
	}
}

// qualifies reports whether the event refers to a file the watcher
// cares about: an interesting write/create/rename/chmod on a file
// with a watched extension that is not a test file.
func (w *watcher) qualifies(ev fsnotify.Event) bool {
	if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
		return false
	}
	return shouldWatch(ev.Name, w.cfg.WatchExts)
}

// addIfNewDir adds path (and its descendants) to the watcher when
// path is a non-excluded directory that fsnotify is not yet
// watching. Called on every Create event; non-directory paths and
// stat errors are silently ignored.
func (w *watcher) addIfNewDir(path string) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	if shouldExcludeDirParts(w.cfg.ProjectRoot, path, w.excName, w.excPath) {
		return
	}
	_ = w.addRecursive(path)
}

// addRecursive walks root and Adds every non-excluded directory.
// fsnotify watches are per-directory and do not recurse on their own.
// Directories that disappear mid-walk (transient race with build
// tools that churn temp dirs) are skipped rather than failing the
// whole walk.
func (w *watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if shouldExcludeDirParts(w.cfg.ProjectRoot, path, w.excName, w.excPath) {
			return fs.SkipDir
		}
		if err := w.fsn.Add(path); err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		return nil
	})
}

// shouldWatch reports whether path's extension is in exts and the
// file is not a Go test file.
func shouldWatch(path string, exts []string) bool {
	base := filepath.Base(path)
	if strings.HasSuffix(base, "_test.go") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(base))
	for _, e := range exts {
		if ext == strings.ToLower(e) {
			return true
		}
	}
	return false
}

// shouldExcludeDir reports whether path (absolute, under root) is
// inside any excluded directory. Bare-name excludes match the
// basename anywhere; path-shaped excludes (containing a separator)
// match exactly that path relative to root, or any descendant of it.
func shouldExcludeDir(root, path string, excludes []string) bool {
	names, paths := splitExcludes(excludes)
	return shouldExcludeDirParts(root, path, names, paths)
}

func shouldExcludeDirParts(root, path string, excName, excPath []string) bool {
	base := filepath.Base(path)
	if slices.Contains(excName, base) {
		return true
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	for _, p := range excPath {
		ps := filepath.ToSlash(p)
		if rel == ps || strings.HasPrefix(rel, ps+"/") {
			return true
		}
	}
	return false
}

// splitExcludes partitions excludes into bare-name vs path-shaped
// entries. The split is purely lexical (separator presence).
func splitExcludes(excludes []string) (names, paths []string) {
	for _, e := range excludes {
		if strings.ContainsRune(e, filepath.Separator) || strings.Contains(e, "/") {
			paths = append(paths, e)
		} else {
			names = append(names, e)
		}
	}
	return names, paths
}
