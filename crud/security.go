package crud

import "github.com/getkin/kin-openapi/openapi3"

// BearerAuth returns an HTTP bearer security scheme for the OpenAPI spec.
// bearerFormat is a free-form hint shown to clients (e.g. "JWT", "opaque
// token"); pass "" to omit it. Register it with [API.AddSecurityScheme].
func BearerAuth(bearerFormat string) *openapi3.SecurityScheme {
	s := openapi3.NewSecurityScheme()
	s.Type = "http"
	s.Scheme = "bearer"
	if bearerFormat != "" {
		s.BearerFormat = bearerFormat
	}
	return s
}

// APIKeyAuth returns an apiKey security scheme carried in `in` ("header",
// "query", or "cookie") under the given key name. Register it with
// [API.AddSecurityScheme].
func APIKeyAuth(in, name string) *openapi3.SecurityScheme {
	s := openapi3.NewSecurityScheme()
	s.Type = "apiKey"
	s.In = in
	s.Name = name
	return s
}

// AddSecurityScheme registers a named security scheme on the document's
// components. Reference the name from [API.Secured] (document-level) or
// [WithSecurity] (per-resource). The scheme is descriptive only — crud does not
// enforce it; authentication stays the host's ordinary middleware.
func (api *API) AddSecurityScheme(name string, scheme *openapi3.SecurityScheme) *API {
	if api.securitySchemes == nil {
		api.securitySchemes = map[string]*openapi3.SecurityScheme{}
	}
	api.securitySchemes[name] = scheme
	return api
}

// Secured sets the document-level security requirement. Multiple scheme names
// are alternatives — satisfying any one authenticates the request, matching
// e.g. "bearer token OR session cookie". A resource can override this with
// [WithSecurity].
func (api *API) Secured(schemeNames ...string) *API {
	api.globalSecurity = append(api.globalSecurity, schemeNames...)
	return api
}

// applySecurity writes the registered schemes and the global requirement onto
// the document.
func (api *API) applySecurity(doc *openapi3.T) {
	if len(api.securitySchemes) > 0 {
		doc.Components.SecuritySchemes = openapi3.SecuritySchemes{}
		for name, scheme := range api.securitySchemes {
			doc.Components.SecuritySchemes[name] = &openapi3.SecuritySchemeRef{Value: scheme}
		}
	}
	if len(api.globalSecurity) > 0 {
		doc.Security = securityRequirements(api.globalSecurity)
	}
}

// securityRequirements builds an OR list of single-scheme requirements: each
// name becomes its own requirement object, so any one satisfies authentication.
func securityRequirements(names []string) openapi3.SecurityRequirements {
	reqs := make(openapi3.SecurityRequirements, 0, len(names))
	for _, name := range names {
		reqs = append(reqs, openapi3.NewSecurityRequirement().Authenticate(name))
	}
	return reqs
}
