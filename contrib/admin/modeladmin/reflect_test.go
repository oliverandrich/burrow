package modeladmin

import (
	"context"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
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

func TestParseUint(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    uint64
		wantErr bool
	}{
		{name: "empty string returns zero", input: "", want: 0},
		{name: "valid uint", input: "42", want: 42},
		{name: "zero", input: "0", want: 0},
		{name: "invalid", input: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUint(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestParseFloat(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{name: "empty string returns zero", input: "", want: 0},
		{name: "valid float", input: "3.14", want: 3.14},
		{name: "integer as float", input: "42", want: 42},
		{name: "zero", input: "0", want: 0},
		{name: "invalid", input: "abc", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFloat(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.InDelta(t, tt.want, got, 0.001)
			}
		})
	}
}

func TestPopulateFromForm_UintField(t *testing.T) {
	type withUint struct {
		Count uint `bun:",notnull"`
	}

	r := newFormRequest(t, url.Values{
		"count": {"7"},
	})

	var item withUint
	err := PopulateFromForm(r, &item)
	require.NoError(t, err)
	assert.Equal(t, uint(7), item.Count)
}

func TestPopulateFromForm_FloatField(t *testing.T) {
	type withFloat struct {
		Price float64 `bun:",notnull"`
	}

	r := newFormRequest(t, url.Values{
		"price": {"9.99"},
	})

	var item withFloat
	err := PopulateFromForm(r, &item)
	require.NoError(t, err)
	assert.InDelta(t, 9.99, item.Price, 0.001)
}

func TestFieldValue(t *testing.T) {
	item := testArticle{
		ID:    42,
		Title: "Hello",
		Views: 100,
	}

	t.Run("existing field", func(t *testing.T) {
		assert.Equal(t, int64(42), FieldValue(item, "ID"))
		assert.Equal(t, "Hello", FieldValue(item, "Title"))
		assert.Equal(t, 100, FieldValue(item, "Views"))
	})

	t.Run("non-existent field", func(t *testing.T) {
		assert.Nil(t, FieldValue(item, "NonExistent"))
	})

	t.Run("pointer to struct", func(t *testing.T) {
		assert.Equal(t, "Hello", FieldValue(&item, "Title"))
	})
}

func TestColumnHTML(t *testing.T) {
	t.Run("string field", func(t *testing.T) {
		item := testArticle{Title: "Hello <World>"}
		got := columnHTML(item, "Title", nil)
		assert.Equal(t, template.HTML("<span>Hello &lt;World&gt;</span>"), got)
	})

	t.Run("int field", func(t *testing.T) {
		item := testArticle{Views: 42}
		got := columnHTML(item, "Views", nil)
		assert.Equal(t, template.HTML("<span>42</span>"), got)
	})

	t.Run("bool field without translator", func(t *testing.T) {
		item := testArticle{Active: true}
		got := columnHTML(item, "Active", nil)
		assert.Equal(t, template.HTML("<span>modeladmin-yes</span>"), got)

		item.Active = false
		got = columnHTML(item, "Active", nil)
		assert.Equal(t, template.HTML("<span>modeladmin-no</span>"), got)
	})

	t.Run("bool field with translator", func(t *testing.T) {
		translator := func(key string) string {
			if key == "modeladmin-yes" {
				return "Yes"
			}
			return "No"
		}
		item := testArticle{Active: true}
		got := columnHTML(item, "Active", translator)
		assert.Equal(t, template.HTML("<span>Yes</span>"), got)
	})

	t.Run("time field", func(t *testing.T) {
		item := testArticle{CreatedAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)}
		got := columnHTML(item, "CreatedAt", nil)
		assert.Equal(t, template.HTML("<span>2024-06-15 10:30</span>"), got)
	})

	t.Run("zero time field", func(t *testing.T) {
		item := testArticle{CreatedAt: time.Time{}}
		got := columnHTML(item, "CreatedAt", nil)
		assert.Equal(t, template.HTML("<span>-</span>"), got)
	})

	t.Run("non-existent field", func(t *testing.T) {
		item := testArticle{}
		got := columnHTML(item, "NonExistent", nil)
		assert.Equal(t, template.HTML("<span>-</span>"), got)
	})

	t.Run("pointer field nil", func(t *testing.T) {
		type withPtr struct {
			Name *string
		}
		item := withPtr{Name: nil}
		got := columnHTML(item, "Name", nil)
		assert.Equal(t, template.HTML("<span>-</span>"), got)
	})

	t.Run("pointer field non-nil", func(t *testing.T) {
		type withPtr struct {
			Name *string
		}
		name := "hello"
		item := withPtr{Name: &name}
		got := columnHTML(item, "Name", nil)
		assert.Equal(t, template.HTML("<span>hello</span>"), got)
	})
}

func TestColumnValue(t *testing.T) {
	item := testArticle{Title: "Test"}
	got := ColumnValue(item, "Title")
	assert.Equal(t, template.HTML("<span>Test</span>"), got)
}

func TestColumnValueFunc(t *testing.T) {
	translator := func(key string) string {
		if key == "modeladmin-yes" {
			return "Ja"
		}
		return "Nein"
	}
	fn := ColumnValueFunc(translator)
	item := testArticle{Active: true}
	got := fn(item, "Active")
	assert.Equal(t, template.HTML("<span>Ja</span>"), got)
}

func TestTableName(t *testing.T) {
	t.Run("with bun table tag", func(t *testing.T) {
		type tagged struct {
			bun.BaseModel `bun:"table:articles"`
			ID            int64 `bun:",pk"`
		}
		assert.Equal(t, "articles", tableName[tagged]())
	})

	t.Run("no bun table tag", func(t *testing.T) {
		type untagged struct {
			ID int64 `bun:",pk"`
		}
		assert.Empty(t, tableName[untagged]())
	})

	t.Run("pointer type", func(t *testing.T) {
		type tagged struct {
			bun.BaseModel `bun:"table:notes"`
			ID            int64 `bun:",pk"`
		}
		assert.Equal(t, "notes", tableName[tagged]())
	})
}
