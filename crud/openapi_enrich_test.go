package crud_test

import (
	"context"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow/crud"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBearerAuthScheme(t *testing.T) {
	s := crud.BearerAuth("JWT")
	assert.Equal(t, "http", s.Type)
	assert.Equal(t, "bearer", s.Scheme)
	assert.Equal(t, "JWT", s.BearerFormat)
}

func TestAPIKeyAuthScheme(t *testing.T) {
	s := crud.APIKeyAuth("header", "X-API-Key")
	assert.Equal(t, "apiKey", s.Type)
	assert.Equal(t, "header", s.In)
	assert.Equal(t, "X-API-Key", s.Name)
}

func TestGlobalSecurity(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "Widgets", Version: "1.0", BaseURL: "/api"})
	api.AddSecurityScheme("bearerAuth", crud.BearerAuth("token"))
	api.Secured("bearerAuth")

	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	require.NotNil(t, doc.Components.SecuritySchemes["bearerAuth"])
	assert.Equal(t, "http", doc.Components.SecuritySchemes["bearerAuth"].Value.Type)

	require.Len(t, doc.Security, 1)
	_, ok := doc.Security[0]["bearerAuth"]
	assert.True(t, ok, "global security must require bearerAuth")
}

func TestSecuredMultipleAreAlternatives(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "Widgets", Version: "1.0", BaseURL: "/api"})
	api.AddSecurityScheme("bearerAuth", crud.BearerAuth("token"))
	api.AddSecurityScheme("cookieAuth", crud.APIKeyAuth("cookie", "session"))
	api.Secured("bearerAuth", "cookieAuth")

	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	// Two separate requirement objects = OR semantics (either satisfies auth).
	require.Len(t, doc.Security, 2)
}

func TestWithSecurityPerResourceOverridePublic(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "Widgets", Version: "1.0", BaseURL: "/api"})
	api.AddSecurityScheme("bearerAuth", crud.BearerAuth("token"))
	api.Secured("bearerAuth")

	r := chi.NewRouter()
	// No scheme names => public, overriding the global requirement.
	api.Mount(r, "/widgets", crud.NewResource[widget](db, crud.WithSecurity[widget]()))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	op := doc.Paths.Find("/widgets").Get
	require.NotNil(t, op.Security)
	assert.Empty(t, *op.Security, "empty security requirement marks the operation public")
}

func TestWithTag(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "Widgets", Version: "1.0", BaseURL: "/api"})
	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db,
		crud.WithTag[widget]("widgets", "Manage widgets in inventory."),
	))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	require.Len(t, doc.Tags, 1)
	assert.Equal(t, "widgets", doc.Tags[0].Name)
	assert.Equal(t, "Manage widgets in inventory.", doc.Tags[0].Description)

	op := doc.Paths.Find("/widgets").Get
	assert.Contains(t, op.Tags, "widgets")
}

func TestWithActionDoc(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "Widgets", Version: "1.0", BaseURL: "/api"})
	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db,
		crud.WithActionDoc[widget](crud.ActionCreate, "Add a widget", "Creates a widget owned by the caller."),
	))

	doc, err := api.Spec()
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	op := doc.Paths.Find("/widgets").Post
	assert.Equal(t, "Add a widget", op.Summary)
	assert.Equal(t, "Creates a widget owned by the caller.", op.Description)
}

func TestWithActionDocEmptySummaryKeepsDefault(t *testing.T) {
	db := newDB(t)
	api := crud.NewAPI(crud.APIInfo{Title: "Widgets", Version: "1.0", BaseURL: "/api"})
	r := chi.NewRouter()
	api.Mount(r, "/widgets", crud.NewResource[widget](db,
		crud.WithActionDoc[widget](crud.ActionCreate, "", "Longer prose only."),
	))

	doc, err := api.Spec()
	require.NoError(t, err)

	op := doc.Paths.Find("/widgets").Post
	assert.NotEmpty(t, op.Summary, "empty summary override keeps the generated default")
	assert.Equal(t, "Longer prose only.", op.Description)
}
