package crud_test

import (
	"net/http"
	"testing"

	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den/where"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReplaceResetsOmittedFields(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "a", Price: 5})
	h := mount(crud.NewResource[widget](db))

	rec := do(t, h, http.MethodPut, "/widgets/"+w.ID, `{"name":"b"}`, nil)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	rec = do(t, h, http.MethodGet, "/widgets/"+w.ID, "", nil)
	var got widget
	decode(t, rec, &got)
	assert.Equal(t, "b", got.Name)
	assert.Equal(t, 0, got.Price, "PUT is a full replace: the omitted field resets to zero")
	assert.Equal(t, w.ID, got.ID, "the id is preserved")
}

func TestReplacePreservesCreatedAt(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "a"})
	h := mount(crud.NewResource[widget](db))

	before := do(t, h, http.MethodGet, "/widgets/"+w.ID, "", nil)
	var orig widget
	decode(t, before, &orig)
	require.False(t, orig.CreatedAt.IsZero())

	require.Equal(t, http.StatusOK,
		do(t, h, http.MethodPut, "/widgets/"+w.ID, `{"name":"b"}`, nil).Code)

	after := do(t, h, http.MethodGet, "/widgets/"+w.ID, "", nil)
	var got widget
	decode(t, after, &got)
	assert.Equal(t, orig.CreatedAt, got.CreatedAt, "full replace must not wipe the creation timestamp")
}

func TestReplaceRespectsScope(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{OwnerID: "alice", Name: "a"})
	other := save(t, db, &widget{OwnerID: "bob", Name: "b"})

	rs := crud.NewResource[widget](db, crud.WithScope[widget](func(r *http.Request) []where.Condition {
		return []where.Condition{where.Field("owner_id").Eq(r.Header.Get("X-Owner"))}
	}))
	// alice tries to replace bob's row -> 404, never reaches it
	rec := do(t, mount(rs), http.MethodPut, "/widgets/"+other.ID, `{"name":"x"}`,
		map[string]string{"X-Owner": "alice"})
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestReplaceUsesCreateWriteModel(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{OwnerID: "alice", Name: "a"})

	type widgetIn struct {
		Name string `json:"name"`
	}
	rs := crud.NewResource[widget](db,
		crud.WithScope[widget](func(r *http.Request) []where.Condition {
			return []where.Condition{where.Field("owner_id").Eq(r.Header.Get("X-Owner"))}
		}),
		crud.WithCreate(func(in widgetIn, r *http.Request) (*widget, error) {
			return &widget{Name: in.Name, OwnerID: r.Header.Get("X-Owner")}, nil
		}),
	)
	h := mount(rs)
	// Body tries to smuggle a different owner; the write model takes owner from
	// the header, so the PUT can't reassign the row.
	rec := do(t, h, http.MethodPut, "/widgets/"+w.ID, `{"name":"b","owner_id":"hacker"}`,
		map[string]string{"X-Owner": "alice"})
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())

	rec = do(t, h, http.MethodGet, "/widgets/"+w.ID, "", map[string]string{"X-Owner": "alice"})
	var got widget
	decode(t, rec, &got)
	assert.Equal(t, "b", got.Name)
	assert.Equal(t, "alice", got.OwnerID, "owner comes from the write model, not the body")
}

func TestReplaceDisabledByExcept(t *testing.T) {
	db := newDB(t)
	w := save(t, db, &widget{Name: "a"})

	rs := crud.NewResource[widget](db, crud.Except[widget](crud.ActionReplace))
	rec := do(t, mount(rs), http.MethodPut, "/widgets/"+w.ID, `{"name":"b"}`, nil)
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code, "Except(ActionReplace) removes PUT")
}

func TestReplaceEnforcesIfMatch(t *testing.T) {
	db := newRevDB(t)
	it := saveRev(t, db)
	h := mountRev(ocResource(db))

	// Missing If-Match -> 428.
	rec := do(t, h, http.MethodPut, "/items/"+it.ID, `{"name":"b"}`, nil)
	assert.Equal(t, http.StatusPreconditionRequired, rec.Code)

	// Matching If-Match -> 200.
	etag := do(t, h, http.MethodGet, "/items/"+it.ID, "", nil).Header().Get("ETag")
	rec = do(t, h, http.MethodPut, "/items/"+it.ID, `{"name":"b"}`, map[string]string{"If-Match": etag})
	assert.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
}
