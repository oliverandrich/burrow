// Package crud turns a Den document type into a standard set of JSON CRUD
// endpoints, so the common 90% of an API can be declared instead of
// hand-written while custom actions stay ordinary chi routes.
//
// A [Resource] is an http.Handler — in the common case, mount it the normal
// chi way:
//
//	r.Route("/api", func(r chi.Router) {
//	    r.Use(auth.RequireAuth())
//	    r.Mount("/notes", crud.NewResource[Note](db, crud.WithScope(ownerScope)))
//	})
//
// When a resource needs custom actions alongside the generated ones, register
// it into a route group with [Resource.Routes] and add the extra routes as
// ordinary siblings:
//
//	r.Route("/api/notes", func(r chi.Router) {
//	    r.Use(auth.RequireAuth())
//	    crud.NewResource[Note](db, crud.WithScope(ownerScope)).Routes(r)
//	    r.Post("/{id}/publish", burrow.Handle(publishNote))
//	})
//
// Authentication, authorization, and CSRF are the host's responsibility via
// ordinary middleware ([auth.RequireAuth], csrf.ExemptPaths); crud is
// representation-agnostic and only speaks JSON.
package crud
