package burrow

import (
	"path/filepath"
	"testing"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type validatedDoc struct {
	document.Base
	Name string `json:"name" validate:"required"`
}

func TestOpenDB_SQLite(t *testing.T) {
	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(t.Context(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping(t.Context()))
}

func TestOpenDB_SQLiteMemory(t *testing.T) {
	db, err := OpenDB(t.Context(), "sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping(t.Context()))
}

func TestOpenDB_UnsupportedScheme(t *testing.T) {
	_, err := OpenDB(t.Context(), "mysql://localhost/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestOpenDB_EmptyDSN(t *testing.T) {
	_, err := OpenDB(t.Context(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestOpenDB_NoScheme(t *testing.T) {
	_, err := OpenDB(t.Context(), "app.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing scheme")
}

// TestOpenDB_ValidationEnabled verifies that OpenDB enables struct-tag
// validation by default: a document with a violated `validate:"required"`
// tag is rejected at insert time.
func TestOpenDB_ValidationEnabled(t *testing.T) {
	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(t.Context(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := t.Context()
	require.NoError(t, den.Register(ctx, db, &validatedDoc{}))

	// Empty Name violates `validate:"required"`.
	doc := &validatedDoc{}
	err = den.Insert(ctx, db, doc)
	require.ErrorIs(t, err, den.ErrValidation)

	// A populated Name passes.
	valid := &validatedDoc{Name: "alice"}
	require.NoError(t, den.Insert(ctx, db, valid))
}

// TestOpenDBWithoutValidation_IsEscapeHatch verifies the migration escape
// hatch: validation is NOT enforced, so a document with violated tags
// inserts successfully.
func TestOpenDBWithoutValidation_IsEscapeHatch(t *testing.T) {
	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDBWithoutValidation(t.Context(), dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := t.Context()
	require.NoError(t, den.Register(ctx, db, &validatedDoc{}))

	// Empty Name would normally violate `validate:"required"` but
	// OpenDBWithoutValidation skips struct-tag checking entirely.
	doc := &validatedDoc{}
	require.NoError(t, den.Insert(ctx, db, doc))
}

func TestOpenStorage_EmptyDSNReturnsNil(t *testing.T) {
	s, err := openStorage(StorageConfig{})
	require.NoError(t, err)
	assert.Nil(t, s)
}

func TestOpenStorage_FileScheme(t *testing.T) {
	// t.TempDir() returns an absolute path. Concatenating with "file://"
	// produces "file:///<abs>" (3 slashes). Under the SQLAlchemy-style
	// convention that is relative; to keep the path absolute we use
	// "file:///" + absPath, which yields "file:////abs/..." (4 slashes).
	dir := t.TempDir()
	s, err := openStorage(StorageConfig{
		DSN: "file:///" + dir + "?url_prefix=/media/",
	})
	require.NoError(t, err)
	require.NotNil(t, s)

	sv, ok := s.(interface{ URLPrefix() string })
	require.True(t, ok, "file.Storage must expose URLPrefix")
	assert.Equal(t, "/media", sv.URLPrefix())
}

func TestOpenStorage_MissingScheme(t *testing.T) {
	_, err := openStorage(StorageConfig{DSN: "./data/media"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing scheme")
}

func TestOpenStorage_UnregisteredScheme(t *testing.T) {
	_, err := openStorage(StorageConfig{DSN: "s3://bucket/prefix"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no backend registered")
}

func TestOpenStorage_FileSchemeWithoutPath(t *testing.T) {
	_, err := openStorage(StorageConfig{DSN: "file://"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "requires a path")
}
