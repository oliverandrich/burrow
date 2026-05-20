package notes

import "github.com/oliverandrich/burrow/contrib/auth"

// Profile extends [auth.User] with a notes-specific display name.
// Stored inline as a nested JSON object on the user document (see
// docs/contrib/auth-profile.md).
type Profile struct {
	Name string `form:"name" verbose:"Display name"`
}

// userDisplayName returns the user's preferred display string for
// rendering — Profile.Name when set, Username as fallback.
func userDisplayName(u *auth.User[Profile]) string {
	if u == nil {
		return ""
	}
	if u.Profile.Name != "" {
		return u.Profile.Name
	}
	return u.Username
}
