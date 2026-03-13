package notes

import (
	"time"

	"github.com/uptrace/bun"

	"github.com/oliverandrich/burrow/contrib/auth"
)

// Note represents a user's note.
type Note struct { //nolint:govet // fieldalignment: readability over optimization
	bun.BaseModel `bun:"table:notes,alias:n"`

	ID        int64      `bun:",pk,autoincrement" json:"id" verbose:"ID"`
	UserID    int64      `bun:",notnull" json:"user_id" form:"-" verbose:"User ID"`
	User      *auth.User `bun:"rel:belongs-to,join:user_id=id" json:"-" form:"-" verbose:"User"`
	Title     string     `bun:",notnull" json:"title" verbose:"Title" form:"title" validate:"required"`
	Content   string     `bun:",notnull,default:''" json:"content" verbose:"Content" form:"content" widget:"textarea"`
	CreatedAt time.Time  `bun:",nullzero,notnull,default:current_timestamp" json:"created_at" form:"-" verbose:"Created at"`
	DeletedAt time.Time  `bun:",soft_delete,nullzero" json:"-" form:"-"`
}
