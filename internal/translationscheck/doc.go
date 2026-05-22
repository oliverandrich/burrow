// Package translationscheck holds the repo-wide translation guards that
// walk every contrib's and example's TOML translation files and assert
// cross-cutting invariants: go-i18n accepts each file (reserved-keys
// guard) and no message ID has divergent values across files for the
// same locale (boot-time merge collision guard).
//
// These tests live in package translationscheck rather than at the
// burrow root because they are build-time integration checks for the
// repository, not part of the burrow library API. They walk the repo
// starting from `../..` (this package's location relative to the
// repository root) to find every `translations/active.*.toml` file.
package translationscheck
