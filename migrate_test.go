package burrow

import (
	"testing"

	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testWidget is a document type for migration/registration tests.
type testWidget struct {
	document.Base
	Name  string `json:"name" den:"index"`
	Color string `json:"color"`
}

func TestRegistryRegisterDocumentsCreatesTable(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	app := &docApp{name: "widgets", docs: []any{&testWidget{}}}
	reg.Add(app)

	err := reg.RegisterDocuments(t.Context(), db)
	require.NoError(t, err)

	// Verify table was created by inserting a document.
	w := &testWidget{Name: "gear", Color: "red"}
	err = den.Insert(t.Context(), db, w)
	require.NoError(t, err)
	assert.NotEmpty(t, w.ID)
}

func TestRegistryRegisterDocumentsSkipsNonDocumentApps(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	reg.Add(&docApp{name: "docapp", docs: []any{&testWidget{}}})
	reg.Add(&minimalApp{}) // Not HasDocuments, should be skipped.

	err := reg.RegisterDocuments(t.Context(), db)
	require.NoError(t, err)

	// Verify the document type from docapp was registered.
	w := &testWidget{Name: "test"}
	err = den.Insert(t.Context(), db, w)
	require.NoError(t, err)
}

func TestRegistryRegisterDocumentsIdempotent(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	app := &docApp{name: "widgets", docs: []any{&testWidget{}}}
	reg.Add(app)

	// Register three times — all should succeed.
	for range 3 {
		err := reg.RegisterDocuments(t.Context(), db)
		require.NoError(t, err)
	}

	// Verify the table works correctly (proves no duplicate errors).
	w := &testWidget{Name: "gear", Color: "red"}
	err := den.Insert(t.Context(), db, w)
	require.NoError(t, err)
}

func TestRegistryRegisterDocumentsMultipleApps(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	// testSetting is a second document type.
	type testSetting struct {
		document.Base
		Key   string `json:"key" den:"unique"`
		Value string `json:"value"`
	}

	reg.Add(&docApp{name: "app_a", docs: []any{&testWidget{}}})
	reg.Add(&docApp{name: "app_b", docs: []any{&testSetting{}}})

	err := reg.RegisterDocuments(t.Context(), db)
	require.NoError(t, err)

	// Verify both document types are registered.
	w := &testWidget{Name: "test"}
	err = den.Insert(t.Context(), db, w)
	require.NoError(t, err)

	s := &testSetting{Key: "theme", Value: "dark"}
	err = den.Insert(t.Context(), db, s)
	require.NoError(t, err)
}

func TestRegistryRegisterDocumentsWithDependencyOrder(t *testing.T) {
	db := testDB(t)
	reg := NewRegistry()

	// Register in dependency order.
	reg.Add(&docApp{name: "base", docs: []any{&testWidget{}}})
	reg.Add(&struct {
		docApp
	}{docApp: docApp{name: "child", docs: nil}})

	err := reg.RegisterDocuments(t.Context(), db)
	require.NoError(t, err)

	// Verify widget table works.
	w := &testWidget{Name: "test"}
	err = den.Insert(t.Context(), db, w)
	require.NoError(t, err)
}
