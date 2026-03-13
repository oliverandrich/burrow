package modeladmin

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

// BulkAction defines a bulk operation on multiple selected items.
type BulkAction struct { //nolint:govet // fieldalignment: readability over optimization
	Slug    string // URL segment: /admin/{model}/bulk/{slug}
	Label   string // button text (i18n key)
	Confirm string // JS confirm() text (i18n key); empty = no confirm
	Handler func(ctx context.Context, db *bun.DB, ids []string) error
}

// RenderBulkAction holds bulk-action metadata for template rendering (no handler).
type RenderBulkAction struct {
	Slug    string
	Label   string
	Confirm string
}

// toRenderBulkAction converts a BulkAction to a template-safe RenderBulkAction.
func (a BulkAction) toRenderBulkAction() RenderBulkAction {
	return RenderBulkAction{
		Slug:    a.Slug,
		Label:   a.Label,
		Confirm: a.Confirm,
	}
}

// DeleteBulkAction returns a BulkAction that deletes selected items by ID.
func DeleteBulkAction[T any]() BulkAction {
	return BulkAction{
		Slug:    "delete",
		Label:   "modeladmin-bulk-delete",
		Confirm: "modeladmin-bulk-delete-confirm",
		Handler: func(ctx context.Context, db *bun.DB, ids []string) error {
			_, err := db.NewDelete().Model((*T)(nil)).Where("id IN (?)", bun.List(ids)).Exec(ctx)
			if err != nil {
				return fmt.Errorf("bulk delete: %w", err)
			}
			return nil
		},
	}
}
