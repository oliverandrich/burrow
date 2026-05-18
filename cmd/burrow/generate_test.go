package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadHostModule(t *testing.T) {
	t.Run("extracts module path", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
			[]byte("module github.com/me/myapp\n\ngo 1.26\n"), 0o644))
		assert.Equal(t, "github.com/me/myapp", readHostModule(dir))
	})

	t.Run("missing go.mod returns empty", func(t *testing.T) {
		assert.Empty(t, readHostModule(t.TempDir()))
	})

	t.Run("go.mod without module line returns empty", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
			[]byte("go 1.26\n"), 0o644))
		assert.Empty(t, readHostModule(dir))
	})

	t.Run("handles indented module line", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"),
			[]byte("  module github.com/me/myapp\n"), 0o644))
		assert.Equal(t, "github.com/me/myapp", readHostModule(dir))
	})
}

func TestValidateAppName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		appName string
		wantErr string
	}{
		{"simple lowercase", "notes", ""},
		{"with digits", "notes2", ""},
		{"underscore", "my_app", ""},
		{"empty", "", "app name is required"},
		{"starts with digit", "2notes", "must start with a letter"},
		{"contains hyphen", "my-app", "may only contain"},
		{"contains uppercase", "Notes", "must start with a lowercase letter"},
		{"contains space", "my app", "may only contain"},
		{"reserved keyword", "package", "is a Go reserved word"},
		{"reserved type", "string", "is a Go reserved word"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := validateAppName(tc.appName)
			if tc.wantErr == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tc.wantErr)
		})
	}
}
