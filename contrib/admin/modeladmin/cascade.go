package modeladmin

import "sync"

// CascadeImpact holds the count of rows that will be cascade-deleted in a child table.
type CascadeImpact struct {
	Table       string // collection name
	DisplayName string // human-readable name
	Count       int
}

// DeleteItem holds per-item information for the delete confirmation page.
type DeleteItem struct {
	ID      string
	Label   string
	Impacts []CascadeImpact
}

// tableDisplayNames maps collection names to human-readable DisplayPluralName values.
// Populated by Init() at boot time for each registered ModelAdmin.
var (
	tableDisplayMu    sync.RWMutex
	tableDisplayNames = make(map[string]string)
)

// RegisterTableDisplayName records a collection → display name mapping.
// This is called automatically by Init() for each ModelAdmin, but can also
// be called manually for collections that don't have their own ModelAdmin.
func RegisterTableDisplayName(table, displayName string) {
	if table == "" || displayName == "" {
		return
	}
	tableDisplayMu.Lock()
	tableDisplayNames[table] = displayName
	tableDisplayMu.Unlock()
}
