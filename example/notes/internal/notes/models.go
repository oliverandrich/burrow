package notes

import (
	"github.com/oliverandrich/den/document"
)

// Note represents a user's note.
type Note struct {
	document.Base
	UserID  string `json:"user_id" den:"index" form:"-" verbose:"User ID"`
	Title   string `json:"title" den:"index,fts" verbose:"Title" form:"title" validate:"required"`
	Content string `json:"content" den:"fts" verbose:"Content" form:"content" widget:"textarea"`
}
