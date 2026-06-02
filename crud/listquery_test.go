package crud_test

import (
	"net/http"
	"testing"

	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/crud"
	"github.com/oliverandrich/den/where"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// names extracts the widget names from a list response, in order.
func names(resp burrow.PageResponse[widget]) []string {
	out := make([]string, len(resp.Items))
	for i, w := range resp.Items {
		out[i] = w.Name
	}
	return out
}

func listResp(t *testing.T, rs *crud.Resource[widget], target string, headers map[string]string) burrow.PageResponse[widget] {
	t.Helper()
	rec := do(t, mount(rs), http.MethodGet, target, "", headers)
	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var resp burrow.PageResponse[widget]
	decode(t, rec, &resp)
	return resp
}

// --- filtering ---

func TestListFilterDisabledByDefault(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a"})
	save(t, db, &widget{Name: "b"})

	// No WithFilter -> query params are ignored, all rows returned.
	resp := listResp(t, crud.NewResource[widget](db), "/widgets?name=a", nil)
	assert.Len(t, resp.Items, 2, "filtering is opt-in; unknown params are ignored")
}

func TestListFilterExactMatch(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a"})
	save(t, db, &widget{Name: "b"})

	rs := crud.NewResource[widget](db, crud.WithFilter[widget]("name"))
	resp := listResp(t, rs, "/widgets?name=a", nil)
	assert.Equal(t, []string{"a"}, names(resp))
	assert.Equal(t, 1, resp.Pagination.TotalCount, "count respects the filter")
}

func TestListFilterIgnoresUnlistedParam(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a", Price: 5})
	save(t, db, &widget{Name: "b", Price: 9})

	// Only "name" is filterable; "price" is not, so ?price is ignored.
	rs := crud.NewResource[widget](db, crud.WithFilter[widget]("name"))
	resp := listResp(t, rs, "/widgets?price=5", nil)
	assert.Len(t, resp.Items, 2, "non-allowlisted param does not filter")
}

func TestListFilterMultiValueIn(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a"})
	save(t, db, &widget{Name: "b"})
	save(t, db, &widget{Name: "c"})

	rs := crud.NewResource[widget](db, crud.WithFilter[widget]("name"))
	resp := listResp(t, rs, "/widgets?name=a&name=c", nil)
	assert.ElementsMatch(t, []string{"a", "c"}, names(resp), "repeated param -> IN match")
}

func TestListFilterCoercesNumericField(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "cheap", Price: 5})
	save(t, db, &widget{Name: "dear", Price: 50})

	rs := crud.NewResource[widget](db, crud.WithFilter[widget]("price"))
	resp := listResp(t, rs, "/widgets?price=50", nil)
	assert.Equal(t, []string{"dear"}, names(resp), "string param coerced to the int field's type")
}

func TestListFilterCoercionFailureIsBadRequest(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a", Price: 5})

	rs := crud.NewResource[widget](db, crud.WithFilter[widget]("price"))
	rec := do(t, mount(rs), http.MethodGet, "/widgets?price=notanumber", "", nil)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "invalid_request")
}

func TestListFilterAndScopeCombined(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{OwnerID: "alice", Name: "keep"})
	save(t, db, &widget{OwnerID: "alice", Name: "drop"})
	save(t, db, &widget{OwnerID: "bob", Name: "keep"})

	rs := crud.NewResource[widget](db,
		crud.WithScope[widget](func(r *http.Request) []where.Condition {
			return []where.Condition{where.Field("owner_id").Eq(r.Header.Get("X-Owner"))}
		}),
		crud.WithFilter[widget]("name"),
	)
	resp := listResp(t, rs, "/widgets?name=keep", map[string]string{"X-Owner": "alice"})
	require.Len(t, resp.Items, 1, "filter is ANDed with scope, never widens it")
	assert.Equal(t, "keep", resp.Items[0].Name)
}

// --- ordering ---

func TestListOrderingDescending(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a", Price: 5})
	save(t, db, &widget{Name: "b", Price: 50})
	save(t, db, &widget{Name: "c", Price: 20})

	rs := crud.NewResource[widget](db, crud.WithOrdering[widget]("price"))
	resp := listResp(t, rs, "/widgets?ordering=-price", nil)
	assert.Equal(t, []string{"b", "c", "a"}, names(resp))
}

func TestListOrderingTieBreaker(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "b", Price: 10})
	save(t, db, &widget{Name: "a", Price: 10})
	save(t, db, &widget{Name: "c", Price: 5})

	rs := crud.NewResource[widget](db, crud.WithOrdering[widget]("price", "name"))
	resp := listResp(t, rs, "/widgets?ordering=price,name", nil)
	assert.Equal(t, []string{"c", "a", "b"}, names(resp), "price asc, then name asc as tie-breaker")
}

func TestListOrderingFallsBackForUnlistedField(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "a", Price: 5})
	save(t, db, &widget{Name: "b", Price: 50})

	// "price" is not orderable, so ?ordering=price is ignored and the default
	// (created-at desc) applies -> most-recently-saved first.
	rs := crud.NewResource[widget](db, crud.WithOrdering[widget]("name"))
	resp := listResp(t, rs, "/widgets?ordering=price", nil)
	assert.Equal(t, []string{"b", "a"}, names(resp), "non-allowlisted field falls back to default sort")
}

// --- search ---

func TestListSearchSubstring(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{Name: "alpha"})
	save(t, db, &widget{Name: "beta"})
	save(t, db, &widget{Name: "alabama"})

	rs := crud.NewResource[widget](db, crud.WithSearch[widget]("name"))
	resp := listResp(t, rs, "/widgets?search=al", nil)
	assert.ElementsMatch(t, []string{"alpha", "alabama"}, names(resp))
}

func TestListSearchIsScoped(t *testing.T) {
	db := newDB(t)
	save(t, db, &widget{OwnerID: "alice", Name: "alpha"})
	save(t, db, &widget{OwnerID: "bob", Name: "alpha"})

	rs := crud.NewResource[widget](db,
		crud.WithScope[widget](func(r *http.Request) []where.Condition {
			return []where.Condition{where.Field("owner_id").Eq(r.Header.Get("X-Owner"))}
		}),
		crud.WithSearch[widget]("name"),
	)
	resp := listResp(t, rs, "/widgets?search=alpha", map[string]string{"X-Owner": "alice"})
	require.Len(t, resp.Items, 1, "search never reaches another owner's rows")
}
