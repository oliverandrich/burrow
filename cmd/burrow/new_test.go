package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
