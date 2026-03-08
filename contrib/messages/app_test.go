package messages

import "codeberg.org/oliverandrich/burrow"

// Compile-time interface assertions.
var (
	_ burrow.App             = (*App)(nil)
	_ burrow.HasDependencies = (*App)(nil)
	_ burrow.HasMiddleware   = (*App)(nil)
)
