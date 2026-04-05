package burrow

import (
	"context"
	"fmt"

	"github.com/oliverandrich/den"
)

// RegisterDocuments registers document types from all HasDocuments apps
// with the Den database. This creates tables and indexes automatically.
func (r *Registry) RegisterDocuments(ctx context.Context, db *den.DB) error {
	for _, app := range r.apps {
		hd, ok := app.(HasDocuments)
		if !ok {
			continue
		}
		if err := den.Register(ctx, db, hd.Documents()...); err != nil {
			return fmt.Errorf("register documents for %q: %w", app.Name(), err)
		}
	}
	return nil
}
