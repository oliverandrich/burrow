package modeladmin

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"time"

	"github.com/oliverandrich/burrow"
)

// HandleExportCSV streams the filtered list as a CSV download.
func (ma *ModelAdmin[T]) HandleExportCSV(w http.ResponseWriter, r *http.Request) error {
	items, err := ma.exportItems(r)
	if err != nil {
		return err
	}

	filename := ma.Slug + "-" + time.Now().Format("2006-01-02") + ".csv"
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	cw := csv.NewWriter(w)

	// Write header row using field names (verbose names require request-time i18n,
	// but export consumers typically want stable machine-readable headers).
	if err := cw.Write(ma.ListFields); err != nil {
		return burrow.NewHTTPError(http.StatusInternalServerError, "failed to write CSV header")
	}

	row := make([]string, len(ma.ListFields))
	for _, item := range items {
		for i, field := range ma.ListFields {
			row[i] = columnText(item, field)
		}
		if err := cw.Write(row); err != nil {
			return burrow.NewHTTPError(http.StatusInternalServerError, "failed to write CSV row")
		}
	}
	cw.Flush()
	return cw.Error()
}

// HandleExportJSON writes the filtered list as a JSON array download.
func (ma *ModelAdmin[T]) HandleExportJSON(w http.ResponseWriter, r *http.Request) error {
	items, err := ma.exportItems(r)
	if err != nil {
		return err
	}

	filename := ma.Slug + "-" + time.Now().Format("2006-01-02") + ".json"
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filename+"\"")

	result := make([]map[string]string, len(items))
	for i, item := range items {
		row := make(map[string]string, len(ma.ListFields))
		for _, field := range ma.ListFields {
			row[field] = columnText(item, field)
		}
		result[i] = row
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

// exportItems builds listOpts from the request and fetches all matching items.
func (ma *ModelAdmin[T]) exportItems(r *http.Request) ([]T, error) {
	opts := listOpts{
		relations:    ma.Relations,
		orderBy:      ma.OrderBy,
		searchTerm:   r.URL.Query().Get("q"),
		searchFields: ma.SearchFields,
		ftsTable:     ma.ftsTable,
		filters:      ma.Filters,
		sortFields:   ma.SortFields,
		r:            r,
	}
	items, err := allItems[T](r.Context(), ma.DB, opts)
	if err != nil {
		return nil, burrow.NewHTTPError(http.StatusInternalServerError, "failed to export items")
	}
	return items, nil
}
