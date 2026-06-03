package crud_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAPISpecStructure(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "Widgets API", Version: "1.0", BaseURL: "/api"})
	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db))

	doc, err := api.Spec()
	require.NoError(t, err)

	// kin-openapi's own validator: the strongest correctness check.
	require.NoError(t, doc.Validate(context.Background()))

	assert.Equal(t, "Widgets API", doc.Info.Title)
	assert.Equal(t, "1.0", doc.Info.Version)
	require.Len(t, doc.Servers, 1)
	assert.Equal(t, "/api", doc.Servers[0].URL)

	// Collection + item paths, all six methods present by default.
	coll := doc.Paths.Value("/widgets")
	require.NotNil(t, coll, "collection path")
	assert.NotNil(t, coll.Get, "list")
	assert.NotNil(t, coll.Post, "create")

	item := doc.Paths.Value("/widgets/{id}")
	require.NotNil(t, item, "item path")
	assert.NotNil(t, item.Get, "get")
	assert.NotNil(t, item.Patch, "update")
	assert.NotNil(t, item.Put, "replace")
	assert.NotNil(t, item.Delete, "delete")

	// The {id} path parameter is declared on the item get.
	var hasID bool
	for _, p := range item.Get.Parameters {
		if p.Value != nil && p.Value.Name == "id" && p.Value.In == "path" {
			hasID = true
		}
	}
	assert.True(t, hasID, "item operations declare the id path parameter")
}

func TestOpenAPIWriteModelAndConstraints(t *testing.T) {
	type widgetIn struct {
		Name string `json:"name" validate:"required,min=3"`
		Note string `json:"note"`
	}

	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "W", Version: "1.0", BaseURL: "/api"})
	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db,
		crud.WithCreate(func(in widgetIn, _ *http.Request) (*widget, error) {
			return &widget{Name: in.Name}, nil
		}),
	))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	body := doc.Paths.Value("/widgets").Post.RequestBody.Value.Content.Get("application/json").Schema
	require.NotNil(t, body.Value, "request body schema resolved")
	assert.Contains(t, body.Value.Required, "name", "validate:required → schema required")
	name := body.Value.Properties["name"]
	require.NotNil(t, name.Value)
	assert.Equal(t, uint64(3), name.Value.MinLength, "validate:min=3 on a string → minLength")
}

func TestOpenAPIListParams(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "W", Version: "1.0", BaseURL: "/api"})
	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db,
		crud.WithFilter[widget]("name", "price"),
		crud.WithOrdering[widget]("name"),
		crud.WithSearch[widget]("name"),
	))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	got := map[string]bool{}
	for _, p := range doc.Paths.Value("/widgets").Get.Parameters {
		if p.Value != nil && p.Value.In == "query" {
			got[p.Value.Name] = true
		}
	}
	for _, want := range []string{"page", "limit", "name", "price", "ordering", "search"} {
		assert.Truef(t, got[want], "list declares ?%s", want)
	}
}

func TestOpenAPIReflectsEnvelopes(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "W", Version: "1.0", BaseURL: "/api"})
	api.Mount(chi.NewRouter(), "/widgets", crud.NewResource[widget](db))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	// The pagination envelope is reflected from burrow.PageResult, so it carries
	// every field — including total_pages, which a hand-built schema had dropped.
	pr := doc.Components.Schemas["PageResult"]
	require.NotNil(t, pr, "PageResult reflected as a component")
	assert.Contains(t, pr.Value.Properties, "total_pages")
	// The error envelope is reflected from the struct handlers emit.
	assert.Contains(t, doc.Components.Schemas, "errorEnvelope")
}

func TestOpenAPIPresenterResponseIsGeneric(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "W", Version: "1.0", BaseURL: "/api"})
	api.Mount(chi.NewRouter(), "/widgets", crud.NewResource[widget](db,
		crud.WithPresenter(func(w *widget) any { return map[string]any{"id": w.ID} }),
	))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	schema := doc.Paths.Value("/widgets/{id}").Get.Responses.Status(http.StatusOK).Value.Content.Get("application/json").Schema
	require.NotNil(t, schema.Value)
	assert.True(t, schema.Value.Type.Is("object"))
	assert.Empty(t, schema.Value.Properties, "a presenter response is a free-form object, not the T schema")
}

func TestOpenAPIRespectsActionSubset(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "RO", Version: "1.0", BaseURL: "/api"})
	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db, crud.Only[widget](crud.ActionList, crud.ActionGet)))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	coll := doc.Paths.Value("/widgets")
	require.NotNil(t, coll)
	assert.NotNil(t, coll.Get, "list kept")
	assert.Nil(t, coll.Post, "create disabled by Only")
	item := doc.Paths.Value("/widgets/{id}")
	require.NotNil(t, item)
	assert.NotNil(t, item.Get, "get kept")
	assert.Nil(t, item.Patch, "update disabled")
	assert.Nil(t, item.Delete, "delete disabled")
}
