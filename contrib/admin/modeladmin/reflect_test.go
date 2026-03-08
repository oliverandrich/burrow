package modeladmin

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testArticle struct { //nolint:govet // fieldalignment: test struct
	ID        int64     `bun:",pk,autoincrement" verbose:"ID"`
	Title     string    `verbose:"Title" form:"required"`
	Body      string    `verbose:"Body" form:"widget=textarea"`
	Status    string    `verbose:"Status" form:"choices=draft|published|archived"`
	Views     int       `verbose:"Views"`
	Active    bool      `verbose:"Active"`
	CreatedAt time.Time `bun:",nullzero,default:current_timestamp" form:"-"`
}

func TestAutoFields_CreateMode(t *testing.T) {
	fields := AutoFields[testArticle](nil)

	// ID should be skipped in create mode (autoincrement PK).
	for _, f := range fields {
		assert.NotEqual(t, "ID", f.Name, "autoincrement PK should be skipped in create mode")
	}

	require.Len(t, fields, 5) // Title, Body, Status, Views, Active (minus ID + CreatedAt)

	// Title
	assert.Equal(t, "Title", fields[0].Name)
	assert.Equal(t, "Title", fields[0].Label)
	assert.Equal(t, "text", fields[0].Type)
	assert.True(t, fields[0].Required)

	// Body
	assert.Equal(t, "Body", fields[1].Name)
	assert.Equal(t, "textarea", fields[1].Type)

	// Status
	assert.Equal(t, "Status", fields[2].Name)
	assert.Equal(t, "select", fields[2].Type)
	assert.Len(t, fields[2].Choices, 3)
	assert.Equal(t, "draft", fields[2].Choices[0].Value)

	// Views
	assert.Equal(t, "Views", fields[3].Name)
	assert.Equal(t, "number", fields[3].Type)

	// Active
	assert.Equal(t, "Active", fields[4].Name)
	assert.Equal(t, "checkbox", fields[4].Type)
}

func TestAutoFields_EditMode(t *testing.T) {
	article := &testArticle{
		ID:     42,
		Title:  "Hello",
		Status: "published",
	}
	fields := AutoFields(article)

	// ID should be present as hidden in edit mode.
	require.Len(t, fields, 6) // ID(hidden) + Title + Body + Status + Views + Active
	assert.Equal(t, "ID", fields[0].Name)
	assert.Equal(t, "hidden", fields[0].Type)
	assert.Equal(t, int64(42), fields[0].Value)

	// Title should have the current value.
	assert.Equal(t, "Hello", fields[1].Value)
}

func TestAutoFields_SkipsFormDash(t *testing.T) {
	fields := AutoFields[testArticle](nil)
	for _, f := range fields {
		assert.NotEqual(t, "CreatedAt", f.Name, "form:\"-\" fields should be skipped")
	}
}

func TestAutoFields_BoolField(t *testing.T) {
	fields := AutoFields[testArticle](nil)
	var found bool
	for _, f := range fields {
		if f.Name == "Active" {
			found = true
			assert.Equal(t, "checkbox", f.Type)
		}
	}
	assert.True(t, found, "Active field should be present")
}

func newFormRequest(t *testing.T, values url.Values) *http.Request {
	t.Helper()
	body := strings.NewReader(values.Encode())
	r, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "/", body)
	require.NoError(t, err)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return r
}

func TestPopulateFromForm(t *testing.T) {
	r := newFormRequest(t, url.Values{
		"title":  {"New Title"},
		"body":   {"Some body text"},
		"status": {"published"},
		"views":  {"100"},
		"active": {"on"},
	})

	var article testArticle
	err := PopulateFromForm(r, &article)
	require.NoError(t, err)

	assert.Equal(t, "New Title", article.Title)
	assert.Equal(t, "Some body text", article.Body)
	assert.Equal(t, "published", article.Status)
	assert.Equal(t, 100, article.Views)
	assert.True(t, article.Active)
	// ID should not be populated (autoincrement PK).
	assert.Equal(t, int64(0), article.ID)
}

func TestPopulateFromForm_SkipsAutoIncrementPK(t *testing.T) {
	r := newFormRequest(t, url.Values{
		"id":    {"999"},
		"title": {"Test"},
	})

	var article testArticle
	err := PopulateFromForm(r, &article)
	require.NoError(t, err)

	assert.Equal(t, int64(0), article.ID, "autoincrement PK should not be populated from form")
}

func TestFormField_FormName(t *testing.T) {
	f := FormField{Name: "Title"}
	assert.Equal(t, "title", f.FormName())

	f = FormField{Name: "CreatedAt"}
	assert.Equal(t, "createdat", f.FormName())
}

func TestInferType(t *testing.T) {
	type sample struct { //nolint:govet // fieldalignment: test struct
		S string
		I int
		B bool
		F float64
		T time.Time
	}
	fields := AutoFields[sample](nil)
	types := map[string]string{}
	for _, f := range fields {
		types[f.Name] = f.Type
	}
	assert.Equal(t, "text", types["S"])
	assert.Equal(t, "number", types["I"])
	assert.Equal(t, "checkbox", types["B"])
	assert.Equal(t, "number", types["F"])
	assert.Equal(t, "date", types["T"])
}

func TestAutoFields_PointerField(t *testing.T) {
	type withPtr struct { //nolint:govet // fieldalignment: test struct
		Name  string
		Email *string `verbose:"Email"`
	}
	email := "test@example.com"
	item := &withPtr{Name: "Test", Email: &email}
	fields := AutoFields(item)
	require.Len(t, fields, 2)
	assert.Equal(t, "test@example.com", *(fields[1].Value.(*string)))
}

func TestPopulateFromForm_PointerField(t *testing.T) {
	type withPtr struct { //nolint:govet // fieldalignment: test struct
		Name  string
		Email *string
	}

	r := newFormRequest(t, url.Values{
		"name":  {"Test"},
		"email": {"test@example.com"},
	})

	var item withPtr
	err := PopulateFromForm(r, &item)
	require.NoError(t, err)
	assert.Equal(t, "Test", item.Name)
	require.NotNil(t, item.Email)
	assert.Equal(t, "test@example.com", *item.Email)
}

func TestVerboseNames(t *testing.T) {
	type tagged struct { //nolint:govet // fieldalignment: test struct
		ID    int64  `verbose:"ID"`
		Name  string `verbose:"Full Name"`
		Plain string
	}

	names := verboseNames[tagged]()
	assert.Equal(t, map[string]string{
		"ID":   "ID",
		"Name": "Full Name",
	}, names)
}

func TestVerboseNames_Empty(t *testing.T) {
	type noTags struct { //nolint:govet // fieldalignment: test struct
		ID   int64
		Name string
	}

	names := verboseNames[noTags]()
	assert.Empty(t, names)
}

func TestPopulateFromForm_EmptyPointerField(t *testing.T) {
	type withPtr struct {
		Email *string
	}

	r := newFormRequest(t, url.Values{
		"email": {""},
	})

	var item withPtr
	err := PopulateFromForm(r, &item)
	require.NoError(t, err)
	assert.Nil(t, item.Email, "empty string should set pointer to nil")
}
