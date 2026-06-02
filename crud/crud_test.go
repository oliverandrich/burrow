package crud_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/burrowtest"
	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den"
	"github.com/oliverandrich/den/document"
	"github.com/oliverandrich/den/where"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// widget is the test document type.
type widget struct {
	document.Base
	OwnerID string `json:"owner_id" den:"index"`
	Name    string `json:"name" validate:"required"`
	Price   int    `json:"price"`
}

func newDB(t *testing.T) *den.DB {
	t.Helper()
	db := burrowtest.DB(t)
	require.NoError(t, den.Register(t.Context(), db, &widget{}))
	return db
}

// save inserts a widget directly (bypassing the API) for arrange steps.
func save(t *testing.T, db *den.DB, w *widget) *widget {
	t.Helper()
	require.NoError(t, den.Save(t.Context(), db, w))
	return w
}

// mount returns a router with the resource mounted at /widgets via r.Mount.
func mount(rs *crud.Resource[widget]) chi.Router {
	r := chi.NewRouter()
	r.Mount("/widgets", rs)
	return r
}

func do(t *testing.T, h http.Handler, method, target, body string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequestWithContext(t.Context(), method, target, rdr)
	req.Header.Set("Accept", "application/json")
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func decode(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), v))
}

func TestList(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a"})
	save(t, db, &widget{Name: "b"})

	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodGet, "/widgets", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var resp burrow.PageResponse[widget]
	decode(t, rec, &resp)
	assert.Len(t, resp.Items, 2)
	assert.Equal(t, 2, resp.Pagination.TotalCount)
}

func TestListPaginated(t *testing.T) {
	db := newDB(t)
	for _, n := range []string{"a", "b", "c"} {
		save(t, db, &widget{Name: n})
	}

	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodGet, "/widgets?limit=2&page=1", "", nil)
	var resp burrow.PageResponse[widget]
	decode(t, rec, &resp)

	assert.Len(t, resp.Items, 2, "limit caps the page")
	assert.Equal(t, 3, resp.Pagination.TotalCount)
	assert.True(t, resp.Pagination.HasMore)
}

func TestListScoped(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{OwnerID: "alice", Name: "a"})
	save(t, db, &widget{OwnerID: "bob", Name: "b"})

	rs := crud.NewResource[widget](db, crud.WithScope[widget](func(r *http.Request) []where.Condition {
		return []where.Condition{where.Field("owner_id").Eq(r.Header.Get("X-Owner"))}
	}))

	rec := do(t, mount(rs), http.MethodGet, "/widgets", "", map[string]string{"X-Owner": "alice"})
	var resp burrow.PageResponse[widget]
	decode(t, rec, &resp)
	require.Len(t, resp.Items, 1, "only the scoped owner's rows")
	assert.Equal(t, "a", resp.Items[0].Name)
}

func TestGet(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "gadget"})

	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodGet, "/widgets/"+w.ID, "", nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	var got widget
	decode(t, rec, &got)
	assert.Equal(t, "gadget", got.Name)
}

func TestGetNotFound(t *testing.T) {
	db := newDB(t)
	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodGet, "/widgets/01ABCNONEXISTENT", "", nil)
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "not_found")
}

func TestGetScopedHidesOthers(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{OwnerID: "alice", Name: "secret"})

	rs := crud.NewResource[widget](db, crud.WithScope[widget](func(r *http.Request) []where.Condition {
		return []where.Condition{where.Field("owner_id").Eq(r.Header.Get("X-Owner"))}
	}))

	// Bob guesses Alice's id — the scope turns it into a 404, not a leak.
	rec := do(t, mount(rs), http.MethodGet, "/widgets/"+w.ID, "", map[string]string{"X-Owner": "bob"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestCreateBindsOntoT(t *testing.T) {
	db := newDB(t)
	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodPost, "/widgets", `{"name":"new","price":9}`, nil)
	assert.Equal(t, http.StatusCreated, rec.Code)

	var got widget
	decode(t, rec, &got)
	assert.Equal(t, "new", got.Name)
	assert.NotEmpty(t, got.ID, "the stored document is returned with its id")
}

func TestCreateValidationError(t *testing.T) {
	db := newDB(t)
	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodPost, "/widgets", `{"price":9}`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)

	var env struct {
		Error struct {
			Code   string            `json:"code"`
			Fields map[string]string `json:"fields"`
		} `json:"error"`
	}
	decode(t, rec, &env)
	assert.Equal(t, "validation_failed", env.Error.Code)
	assert.Contains(t, env.Error.Fields, "name", "the failing field is named")
}

func TestCreateBadJSON(t *testing.T) {
	db := newDB(t)
	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodPost, "/widgets", `{not json`, nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

// createInput is a write DTO that excludes server-owned fields.
type createInput struct {
	Name  string `json:"name" validate:"required"`
	Price int    `json:"price"`
}

func TestCreateWithDTOBlocksMassAssignment(t *testing.T) {
	db := newDB(t)
	rs := crud.NewResource[widget](db, crud.WithCreate(func(in createInput, _ *http.Request) (*widget, error) {
		return &widget{OwnerID: "system", Name: in.Name, Price: in.Price}, nil
	}))

	// The client tries to set owner_id; the DTO doesn't carry it, so it's ignored.
	rec := do(t, mount(rs), http.MethodPost, "/widgets", `{"name":"x","owner_id":"hacker"}`, nil)
	require.Equal(t, http.StatusCreated, rec.Code)

	var got widget
	decode(t, rec, &got)
	assert.Equal(t, "system", got.OwnerID, "server-owned field is not client-settable")
}

func TestUpdate(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "old", Price: 1})

	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodPatch, "/widgets/"+w.ID, `{"name":"updated","price":2}`, nil)
	assert.Equal(t, http.StatusOK, rec.Code)

	got, err := den.FindByID[widget](t.Context(), db, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "updated", got.Name)
	assert.Equal(t, 2, got.Price)
}

func TestPatchMergesPutReplaces(t *testing.T) {
	db := newDB(t)
	h := mount(crud.NewResource[widget](db))

	// PATCH leaves the omitted field; PUT resets it — the two verbs differ.
	patched := save(t, db, &widget{Name: "old", Price: 5})
	require.Equal(t, http.StatusOK,
		do(t, h, http.MethodPatch, "/widgets/"+patched.ID, `{"name":"new"}`, nil).Code)
	got, err := den.FindByID[widget](t.Context(), db, patched.ID)
	require.NoError(t, err)
	assert.Equal(t, 5, got.Price, "PATCH preserves the omitted field")

	replaced := save(t, db, &widget{Name: "old", Price: 5})
	require.Equal(t, http.StatusOK,
		do(t, h, http.MethodPut, "/widgets/"+replaced.ID, `{"name":"new"}`, nil).Code)
	got, err = den.FindByID[widget](t.Context(), db, replaced.ID)
	require.NoError(t, err)
	assert.Equal(t, 0, got.Price, "PUT replaces, resetting the omitted field")
}

func TestUpdatePartialMergePreservesOmittedFields(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "old", Price: 5})

	// PATCH only the name; price is omitted and must be preserved.
	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodPatch, "/widgets/"+w.ID, `{"name":"new"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	got, err := den.FindByID[widget](t.Context(), db, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "new", got.Name)
	assert.Equal(t, 5, got.Price, "omitted field is preserved (partial merge)")
}

func TestUpdateWithDTO(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{OwnerID: "alice", Name: "old"})

	rs := crud.NewResource[widget](db, crud.WithUpdate(func(in createInput, dst *widget, _ *http.Request) error {
		dst.Name = in.Name
		return nil
	}))

	rec := do(t, mount(rs), http.MethodPatch, "/widgets/"+w.ID, `{"name":"renamed","owner_id":"hacker"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	got, err := den.FindByID[widget](t.Context(), db, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name)
	assert.Equal(t, "alice", got.OwnerID, "the DTO can't touch server-owned fields")
}

// TestUpdateWithPointerDTOPartial shows the supported partial-update pattern:
// a pointer-field DTO whose mapper only applies fields the client actually sent.
func TestUpdateWithPointerDTOPartial(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "old", Price: 7})

	type patchInput struct {
		Name  *string `json:"name"`
		Price *int    `json:"price"`
	}
	rs := crud.NewResource[widget](db, crud.WithUpdate(func(in patchInput, dst *widget, _ *http.Request) error {
		if in.Name != nil {
			dst.Name = *in.Name
		}
		if in.Price != nil {
			dst.Price = *in.Price
		}
		return nil
	}))

	// PATCH only the name; price is absent → mapper leaves it alone.
	rec := do(t, mount(rs), http.MethodPatch, "/widgets/"+w.ID, `{"name":"renamed"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code)

	got, err := den.FindByID[widget](t.Context(), db, w.ID)
	require.NoError(t, err)
	assert.Equal(t, "renamed", got.Name)
	assert.Equal(t, 7, got.Price, "pointer DTO leaves omitted fields untouched")
}

func TestDelete(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "doomed"})

	rec := do(t, mount(crud.NewResource[widget](db)), http.MethodDelete, "/widgets/"+w.ID, "", nil)
	assert.Equal(t, http.StatusNoContent, rec.Code)

	_, err := den.FindByID[widget](t.Context(), db, w.ID)
	require.ErrorIs(t, err, den.ErrNotFound)
}

func TestPresenter(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "gadget", Price: 5})

	rs := crud.NewResource[widget](db, crud.WithPresenter(func(w *widget) any {
		return map[string]any{"label": w.Name}
	}))

	rec := do(t, mount(rs), http.MethodGet, "/widgets", "", nil)
	var resp struct {
		Items []map[string]any `json:"items"`
	}
	decode(t, rec, &resp)
	require.Len(t, resp.Items, 1)
	assert.Equal(t, "gadget", resp.Items[0]["label"])
	assert.NotContains(t, resp.Items[0], "price", "only presenter fields are exposed")
}

func TestOnlyDisablesActions(t *testing.T) {
	db := newDB(t)
	rs := crud.NewResource[widget](db, crud.Only[widget](crud.ActionList, crud.ActionGet))

	rec := do(t, mount(rs), http.MethodPost, "/widgets", `{"name":"x"}`, nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "create is not registered")
}

func TestRoutesWithCustomSibling(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "gadget"})

	r := chi.NewRouter()
	r.Route("/widgets", func(r chi.Router) {
		crud.NewResource[widget](db).Routes(r)
		r.Post("/{id}/publish", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
			return burrow.JSON(w, http.StatusOK, map[string]string{"published": chi.URLParam(r, "id")})
		}))
	})

	// Generated action still works.
	assert.Equal(t, http.StatusOK, do(t, r, http.MethodGet, "/widgets/"+w.ID, "", nil).Code)

	// Custom sibling coexists.
	rec := do(t, r, http.MethodPost, "/widgets/"+w.ID+"/publish", "", nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), w.ID)
}
