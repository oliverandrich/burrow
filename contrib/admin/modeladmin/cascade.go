package modeladmin

import (
	"context"
	"fmt"
	"sync"

	"github.com/uptrace/bun"
)

// cascadeRef describes a foreign key with ON DELETE CASCADE that references our table.
type cascadeRef struct {
	Table  string // child table containing the FK
	Column string // FK column in the child table
}

// CascadeImpact holds the count of rows that will be cascade-deleted in a child table.
type CascadeImpact struct {
	Table       string // SQL table name
	DisplayName string // human-readable name (from registered ModelAdmin, or Table as fallback)
	Count       int
}

// DeleteItem holds per-item information for the delete confirmation page.
type DeleteItem struct {
	ID      string
	Label   string
	Impacts []CascadeImpact
}

// tableDisplayNames maps SQL table names to human-readable DisplayPluralName values.
// Populated by Init() at boot time for each registered ModelAdmin.
var (
	tableDisplayMu    sync.RWMutex
	tableDisplayNames = make(map[string]string)
)

// RegisterTableDisplayName records a table → display name mapping.
// This is called automatically by Init() for each ModelAdmin, but can also
// be called manually for tables that don't have their own ModelAdmin
// (e.g. internal tables like credentials or recovery_codes).
func RegisterTableDisplayName(table, displayName string) {
	if table == "" || displayName == "" {
		return
	}
	tableDisplayMu.Lock()
	tableDisplayNames[table] = displayName
	tableDisplayMu.Unlock()
}

// lookupTableDisplayName returns the display name for a table, or the raw table name as fallback.
func lookupTableDisplayName(table string) string {
	tableDisplayMu.RLock()
	name, ok := tableDisplayNames[table]
	tableDisplayMu.RUnlock()
	if ok {
		return name
	}
	return table
}

// detectCascades introspects SQLite foreign keys to find tables that reference
// targetTable with ON DELETE CASCADE. Called once at boot time.
func detectCascades(db *bun.DB, targetTable string) []cascadeRef {
	if targetTable == "" {
		return nil
	}

	ctx := context.Background()

	// Get all table names from sqlite_master.
	var tables []string
	err := db.NewRaw("SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE '\\_%' ESCAPE '\\'").Scan(ctx, &tables)
	if err != nil {
		return nil
	}

	var refs []cascadeRef
	for _, tbl := range tables {
		if r := scanForeignKeys(ctx, db, tbl, targetTable); len(r) > 0 {
			refs = append(refs, r...)
		}
	}

	return refs
}

// scanForeignKeys reads the foreign key list for a single table and returns
// cascade references that target the given table.
func scanForeignKeys(ctx context.Context, db *bun.DB, tbl, targetTable string) []cascadeRef {
	rows, err := db.QueryContext(ctx, fmt.Sprintf("PRAGMA foreign_key_list(%q)", tbl)) //nolint:gosec // tbl comes from sqlite_master, not user input
	if err != nil {
		return nil
	}
	defer func() { _ = rows.Close() }()

	var refs []cascadeRef
	for rows.Next() {
		var (
			id, seq                                int
			table, from, to, onUpdate, onDelete, m string
		)
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &m); err != nil {
			continue
		}
		if table == targetTable && onDelete == "CASCADE" {
			refs = append(refs, cascadeRef{Table: tbl, Column: from})
		}
	}
	return refs
}

// countPerItemCascadeImpacts counts cascade impacts grouped by parent ID.
// Returns a map from parent ID to its cascade impacts (only non-zero counts).
func countPerItemCascadeImpacts(ctx context.Context, db *bun.DB, cascades []cascadeRef, ids []string) (map[string][]CascadeImpact, error) {
	result := make(map[string][]CascadeImpact)
	for _, c := range cascades {
		if err := countCascadeForRef(ctx, db, c, ids, result); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// countCascadeForRef counts cascade impacts for a single cascade reference, grouped by parent ID.
func countCascadeForRef(ctx context.Context, db *bun.DB, c cascadeRef, ids []string, result map[string][]CascadeImpact) error {
	rows, err := db.QueryContext(ctx,
		fmt.Sprintf("SELECT %q, COUNT(*) FROM %q WHERE %q IN (?) GROUP BY %q", c.Column, c.Table, c.Column, c.Column), //nolint:gosec // table/column from boot-time introspection
		bun.List(ids),
	)
	if err != nil {
		return fmt.Errorf("count cascade impact for %s.%s: %w", c.Table, c.Column, err)
	}
	defer func() { _ = rows.Close() }()

	displayName := lookupTableDisplayName(c.Table)
	for rows.Next() {
		var parentID string
		var count int
		if err := rows.Scan(&parentID, &count); err != nil {
			return fmt.Errorf("scan cascade impact for %s.%s: %w", c.Table, c.Column, err)
		}
		if count > 0 {
			result[parentID] = append(result[parentID], CascadeImpact{
				Table:       c.Table,
				DisplayName: displayName,
				Count:       count,
			})
		}
	}
	return nil
}
