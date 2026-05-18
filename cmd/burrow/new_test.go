package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextStepsMessage(t *testing.T) {
	t.Run("with mise present", func(t *testing.T) {
		got := nextStepsMessage("warren", "warren", true)
		assert.Contains(t, got, "mise run setup")
		assert.Contains(t, got, "mise run dev")
		assert.NotContains(t, got, "go mod tidy")
		assert.NotContains(t, got, "go run ./cmd/warren")
	})

	t.Run("without mise", func(t *testing.T) {
		got := nextStepsMessage("warren", "warren", false)
		assert.Contains(t, got, "go mod tidy")
		assert.Contains(t, got, "go run ./cmd/warren")
		assert.NotContains(t, got, "mise")
	})

	t.Run("destDir is interpolated", func(t *testing.T) {
		got := nextStepsMessage("path/to/myapp", "myapp", true)
		assert.Contains(t, got, "cd path/to/myapp")
	})
}

func TestBootstrapGit(t *testing.T) {
	ctx := context.Background()

	t.Run("git present, init succeeds", func(t *testing.T) {
		var initCalledWith string
		bootstrapGit(ctx, "/some/dir", io.Discard,
			func() bool { return true },
			func(_ context.Context, dir string) error { initCalledWith = dir; return nil },
		)
		assert.Equal(t, "/some/dir", initCalledWith)
	})

	t.Run("git present, init fails", func(t *testing.T) {
		var buf bytes.Buffer
		bootstrapGit(ctx, "/some/dir", &buf,
			func() bool { return true },
			func(context.Context, string) error { return errors.New("permission denied") },
		)
		assert.Contains(t, buf.String(), "git init failed")
		assert.Contains(t, buf.String(), "permission denied")
	})

	t.Run("git missing", func(t *testing.T) {
		var buf bytes.Buffer
		initCalled := false
		bootstrapGit(ctx, "/some/dir", &buf,
			func() bool { return false },
			func(context.Context, string) error { initCalled = true; return nil },
		)
		assert.False(t, initCalled, "init must not be called when git is missing")
		assert.Contains(t, buf.String(), "git not on PATH")
	})
}

func TestGuessGitUser(t *testing.T) {
	for _, tc := range []struct {
		name       string
		modulePath string
		want       string
	}{
		{"github org/repo", "github.com/oliverandrich/burrow", "oliverandrich"},
		{"gitlab nested groups", "gitlab.com/group/sub/repo", "group"},
		{"local module name only", "myapp", ""},
		{"trailing slash", "github.com/me/repo/", "me"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, guessGitUser(tc.modulePath))
		})
	}
}

func TestResolveAuthor(t *testing.T) {
	for _, tc := range []struct {
		name    string
		gitUser string
		gitName string
		want    string
	}{
		{"git config wins", "oliverandrich", "Oliver Andrich", "Oliver Andrich"},
		{"empty git config falls back to gitUser", "acme-corp", "", "acme-corp"},
		{"both empty yields empty", "", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveAuthor(tc.gitUser, func() string { return tc.gitName })
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestValidateModulePath(t *testing.T) {
	t.Run("valid path", func(t *testing.T) {
		assert.NoError(t, validateModulePath("github.com/me/myapp"))
	})

	for _, tc := range []struct {
		name       string
		modulePath string
		wantErr    string
	}{
		{"empty", "", "--module is required"},
		{"contains space", "github.com/me/my app", "must not contain spaces"},
		{"no slash", "myapp", "must contain at least one slash"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateModulePath(tc.modulePath)
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
