package admin

import "github.com/oliverandrich/burrow"

// NavGroup groups navigation items belonging to one admin app.
// Each HasAdmin app contributes one group to the admin sidebar.
type NavGroup struct {
	AppName string
	Items   []burrow.NavItem
}
