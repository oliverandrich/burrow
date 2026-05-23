package dev

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestShouldWatch(t *testing.T) {
	tests := []struct {
		name string
		path string
		exts []string
		want bool
	}{
		{"go file", "foo.go", []string{".go", ".html"}, true},
		{"html file", "templates/x.html", []string{".go", ".html"}, true},
		{"unwatched ext", "README.md", []string{".go", ".html"}, false},
		{"test file ignored", "foo_test.go", []string{".go"}, false},
		{"case-insensitive ext", "STYLE.CSS", []string{".css"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldWatch(tt.path, tt.exts))
		})
	}
}

func TestShouldExcludeDir(t *testing.T) {
	excludes := []string{".git", "node_modules", filepath.Join("internal", "app", "static")}
	root := "/some/project"

	tests := []struct {
		path string
		want bool
	}{
		{filepath.Join(root, ".git"), true},
		{filepath.Join(root, "src", ".git"), true}, // bare name matches anywhere
		{filepath.Join(root, "node_modules"), true},
		{filepath.Join(root, "internal", "app", "static"), true},         // path-shaped match
		{filepath.Join(root, "internal", "app", "static", "deep"), true}, // descendants too
		{filepath.Join(root, "internal", "app"), false},
		{filepath.Join(root, "cmd"), false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, shouldExcludeDir(root, tt.path, excludes))
		})
	}
}

func TestWatcher_FiresDebouncedEventOnWrite(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal"), 0o755))

	cfg := Config{
		ProjectRoot: root,
		WatchExts:   []string{".go"},
		ExcludeDirs: defaultExcludeDirs,
		Debounce:    100 * time.Millisecond,
	}
	w, err := newWatcher(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	events := make(chan struct{}, 8)
	go w.Run(t.Context(), func() { events <- struct{}{} })

	// Burst of writes inside the debounce window should collapse into one event.
	for range 5 {
		require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "x.go"), []byte("package x\n"), 0o644))
		time.Sleep(10 * time.Millisecond)
	}

	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("expected at least one restart event from the watcher")
	}

	// No second event should arrive after the burst settles.
	select {
	case <-events:
		// Drain anything still queued from the original burst, but require
		// the channel to be quiet thereafter.
	case <-time.After(50 * time.Millisecond):
	}
	select {
	case <-events:
		t.Fatal("debounced watcher fired more than once for a single burst")
	case <-time.After(300 * time.Millisecond):
	}
}

func TestWatcher_IgnoresExcludedDir(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "tmp"), 0o755))

	cfg := Config{
		ProjectRoot: root,
		WatchExts:   []string{".go"},
		ExcludeDirs: []string{"tmp"},
		Debounce:    50 * time.Millisecond,
	}
	w, err := newWatcher(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	events := make(chan struct{}, 1)
	go w.Run(t.Context(), func() { events <- struct{}{} })

	// Writing into an excluded dir must not trigger an event.
	require.NoError(t, os.WriteFile(filepath.Join(root, "tmp", "x.go"), []byte("package x\n"), 0o644))

	select {
	case <-events:
		t.Fatal("watcher fired on a write inside an excluded directory")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestWatcher_AddsNewlyCreatedDir(t *testing.T) {
	root := t.TempDir()

	cfg := Config{
		ProjectRoot: root,
		WatchExts:   []string{".go"},
		ExcludeDirs: defaultExcludeDirs,
		Debounce:    100 * time.Millisecond,
	}
	w, err := newWatcher(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	events := make(chan struct{}, 4)
	go w.Run(t.Context(), func() { events <- struct{}{} })

	// Drain the create event for the new directory itself (which is
	// not a watched-extension write but does flow through qualify).
	require.NoError(t, os.MkdirAll(filepath.Join(root, "pkg"), 0o755))
	// Give the watcher a beat to pick up the new directory.
	time.Sleep(150 * time.Millisecond)

	require.NoError(t, os.WriteFile(filepath.Join(root, "pkg", "x.go"), []byte("package x\n"), 0o644))

	select {
	case <-events:
	case <-time.After(time.Second):
		t.Fatal("watcher did not fire on a write inside a directory created after startup")
	}
}

func TestWatcher_IgnoresTestFiles(t *testing.T) {
	root := t.TempDir()

	cfg := Config{
		ProjectRoot: root,
		WatchExts:   []string{".go"},
		ExcludeDirs: defaultExcludeDirs,
		Debounce:    50 * time.Millisecond,
	}
	w, err := newWatcher(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	events := make(chan struct{}, 1)
	go w.Run(t.Context(), func() { events <- struct{}{} })

	require.NoError(t, os.WriteFile(filepath.Join(root, "foo_test.go"), []byte("package main\n"), 0o644))

	select {
	case <-events:
		t.Fatal("watcher fired on a *_test.go write")
	case <-time.After(200 * time.Millisecond):
	}
}
