package modeladmin

import (
	"html/template"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

type testArticle struct { //nolint:govet // fieldalignment: test struct
	ID        int64     `bun:",pk,autoincrement" verbose:"ID"`
	Title     string    `verbose:"Title"`
	Body      string    `verbose:"Body"`
	Status    string    `verbose:"Status"`
	Views     int       `verbose:"Views"`
	Active    bool      `verbose:"Active"`
	CreatedAt time.Time `bun:",nullzero,default:current_timestamp"`
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
