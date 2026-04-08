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
	db, err := OpenDB(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping(t.Context()))
}

func TestOpenDB_SQLiteMemory(t *testing.T) {
	db, err := OpenDB("sqlite://:memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.Ping(t.Context()))
}

func TestOpenDB_UnsupportedScheme(t *testing.T) {
	_, err := OpenDB("mysql://localhost/test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestOpenDB_EmptyDSN(t *testing.T) {
	_, err := OpenDB("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestOpenDB_NoScheme(t *testing.T) {
	_, err := OpenDB("app.db")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing scheme")
}

// TestOpenDB_ValidationEnabled verifies that OpenDB enables struct-tag
// validation by default: a document with a violated `validate:"required"`
// tag is rejected at insert time.
func TestOpenDB_ValidationEnabled(t *testing.T) {
	dsn := "sqlite:///" + filepath.Join(t.TempDir(), "test.db")
	db, err := OpenDB(dsn)
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
	db, err := OpenDBWithoutValidation(dsn)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	ctx := t.Context()
	require.NoError(t, den.Register(ctx, db, &validatedDoc{}))

	// Empty Name would normally violate `validate:"required"` but
	// OpenDBWithoutValidation skips struct-tag checking entirely.
	doc := &validatedDoc{}
	require.NoError(t, den.Insert(ctx, db, doc))
}
