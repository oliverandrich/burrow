package dev

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// EnsureEnv creates the configured env file with default dev keys
// when it does not yet exist; existing files are left untouched. It
// is the entry point for `burrow dev --init-env`, called by the
// scaffold's `mise run setup` task so freshly-cloned projects have a
// `.env` ready before any test or app invocation. Returns an error
// when cfg.EnvFile is empty.
func EnsureEnv(cfg Config, log io.Writer) error {
	if cfg.EnvFile == "" {
		return errors.New("dev: EnsureEnv requires a non-empty EnvFile")
	}
	_, err := loadOrGenerateEnv(cfg, log)
	return err
}

// loadOrGenerateEnv resolves cfg.EnvFile and returns its KEY=VALUE
// pairs. Relative paths resolve against cfg.ProjectRoot; absolute
// paths are used verbatim. When the file is missing it is
// auto-created with conventional dev defaults — see
// [generateDefaultEnvFile] — and a one-line notice is written to log.
// When cfg.EnvFile is empty the function returns an empty map and no
// error: the caller treats env injection as disabled.
func loadOrGenerateEnv(cfg Config, log io.Writer) (map[string]string, error) {
	if cfg.EnvFile == "" {
		return nil, nil
	}
	path := cfg.EnvFile
	if !filepath.IsAbs(path) {
		path = filepath.Join(cfg.ProjectRoot, path)
	}

	if _, err := os.Stat(path); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("dev: stat %s: %w", path, err)
		}
		if err := generateDefaultEnvFile(path); err != nil {
			return nil, err
		}
		logf(log, "burrow dev: generated %s with default dev keys\n", cfg.EnvFile)
	}

	vars, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("dev: parse %s: %w", path, err)
	}
	return vars, nil
}

// generateDefaultEnvFile writes a fresh env file at path containing
// SESSION_HASH_KEY and CSRF_KEY entries with 32-byte hex-encoded
// random values. These are the secrets that contrib/session and
// contrib/csrf expect; builds that do not include those contribs
// inherit the file harmlessly.
func generateDefaultEnvFile(path string) error {
	sessionKey, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("dev: generate session key: %w", err)
	}
	csrfKey, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("dev: generate csrf key: %w", err)
	}
	content := fmt.Sprintf("SESSION_HASH_KEY=%s\nCSRF_KEY=%s\n", sessionKey, csrfKey)
	return os.WriteFile(path, []byte(content), 0o600)
}

func randomHex(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// mergeEnv produces a child-process environment by overlaying
// overrides onto base, with the shell-wins precedence used by every
// standard dotenv tool: a key present in base is kept; the override
// only takes effect when base has no value for the key. This lets the
// developer ad-hoc override an env-file value via `KEY=val burrow dev`
// without editing the file.
func mergeEnv(base []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base))
	for _, kv := range base {
		if key, _, ok := strings.Cut(kv, "="); ok {
			seen[key] = true
		}
	}
	out := make([]string, 0, len(base)+len(overrides))
	out = append(out, base...)
	for k, v := range overrides {
		if !seen[k] {
			out = append(out, k+"="+v)
		}
	}
	return out
}
