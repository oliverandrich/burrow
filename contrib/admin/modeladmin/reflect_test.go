package modeladmin

import (
	"fmt"
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

// testCategory implements fmt.Stringer for FK label testing.
type testCategory struct { //nolint:govet // fieldalignment: test struct
	ID   int64
	Name string
}

func (c testCategory) String() string { return c.Name }

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
		got := columnHTML(item, "Title", nil, nil)
		assert.Equal(t, template.HTML("<span>Hello &lt;World&gt;</span>"), got)
	})

	t.Run("int field", func(t *testing.T) {
		item := testArticle{Views: 42}
		got := columnHTML(item, "Views", nil, nil)
		assert.Equal(t, template.HTML("<span>42</span>"), got)
	})

	t.Run("bool field without translator", func(t *testing.T) {
		item := testArticle{Active: true}
		got := columnHTML(item, "Active", nil, nil)
		assert.Equal(t, template.HTML("<span>modeladmin-yes</span>"), got)

		item.Active = false
		got = columnHTML(item, "Active", nil, nil)
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
		got := columnHTML(item, "Active", translator, nil)
		assert.Equal(t, template.HTML("<span>Yes</span>"), got)
	})

	t.Run("time field", func(t *testing.T) {
		item := testArticle{CreatedAt: time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)}
		got := columnHTML(item, "CreatedAt", nil, nil)
		assert.Equal(t, template.HTML("<span>2024-06-15 10:30</span>"), got)
	})

	t.Run("zero time field", func(t *testing.T) {
		item := testArticle{CreatedAt: time.Time{}}
		got := columnHTML(item, "CreatedAt", nil, nil)
		assert.Equal(t, template.HTML("<span>-</span>"), got)
	})

	t.Run("non-existent field", func(t *testing.T) {
		item := testArticle{}
		got := columnHTML(item, "NonExistent", nil, nil)
		assert.Equal(t, template.HTML("<span>-</span>"), got)
	})

	t.Run("pointer field nil", func(t *testing.T) {
		type withPtr struct {
			Name *string
		}
		item := withPtr{Name: nil}
		got := columnHTML(item, "Name", nil, nil)
		assert.Equal(t, template.HTML("<span>-</span>"), got)
	})

	t.Run("pointer field non-nil", func(t *testing.T) {
		type withPtr struct {
			Name *string
		}
		name := "hello"
		item := withPtr{Name: &name}
		got := columnHTML(item, "Name", nil, nil)
		assert.Equal(t, template.HTML("<span>hello</span>"), got)
	})

	t.Run("stringer field shows String() result", func(t *testing.T) {
		type article struct {
			Category *testCategory
		}
		item := article{Category: &testCategory{ID: 1, Name: "Science"}}
		got := columnHTML(item, "Category", nil, nil)
		assert.Equal(t, template.HTML("<span>Science</span>"), got)
	})

	t.Run("stringer field nil pointer shows dash", func(t *testing.T) {
		type article struct {
			Category *testCategory
		}
		item := article{Category: nil}
		got := columnHTML(item, "Category", nil, nil)
		assert.Equal(t, template.HTML("<span>-</span>"), got)
	})

	t.Run("stringer field with HTML is escaped", func(t *testing.T) {
		type article struct {
			Category *testCategory
		}
		item := article{Category: &testCategory{ID: 1, Name: "<b>Bold</b>"}}
		got := columnHTML(item, "Category", nil, nil)
		assert.Equal(t, template.HTML("<span>&lt;b&gt;Bold&lt;/b&gt;</span>"), got)
	})

	t.Run("stringer value type field", func(t *testing.T) {
		type article struct {
			Category testCategory
		}
		item := article{Category: testCategory{ID: 1, Name: "Tech"}}
		got := columnHTML(item, "Category", nil, nil)
		assert.Equal(t, template.HTML("<span>Tech</span>"), got)
	})

	t.Run("computed column takes priority", func(t *testing.T) {
		item := testArticle{Title: "Original"}
		computed := map[string]func(any) template.HTML{
			"Title": func(item any) template.HTML {
				return "<span>Computed</span>"
			},
		}
		got := columnHTML(item, "Title", nil, computed)
		assert.Equal(t, template.HTML("<span>Computed</span>"), got)
	})

	t.Run("computed column for non-existent field", func(t *testing.T) {
		item := testArticle{}
		computed := map[string]func(any) template.HTML{
			"Custom": func(item any) template.HTML {
				return "<span>custom value</span>"
			},
		}
		got := columnHTML(item, "Custom", nil, computed)
		assert.Equal(t, template.HTML("<span>custom value</span>"), got)
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
	fn := ColumnValueFunc(translator, nil)
	item := testArticle{Active: true}
	got := fn(item, "Active")
	assert.Equal(t, template.HTML("<span>Ja</span>"), got)
}

func TestColumnValueFunc_WithComputed(t *testing.T) {
	computed := map[string]func(any) template.HTML{
		"ChoiceCount": func(item any) template.HTML {
			return "<span>5 choices</span>"
		},
	}
	fn := ColumnValueFunc(nil, computed)

	item := testArticle{}
	got := fn(item, "ChoiceCount")
	assert.Equal(t, template.HTML("<span>5 choices</span>"), got)
}

// Verify that testCategory satisfies fmt.Stringer at compile time.
var _ fmt.Stringer = testCategory{}

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
