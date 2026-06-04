package crud

import (
	"net/http"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
)

// APIInfo carries the document-level metadata for a generated OpenAPI spec.
// BaseURL becomes the spec's single server, so resource paths stay relative.
type APIInfo struct {
	Title       string
	Version     string
	Description string
	BaseURL     string
}

// Mountable is the set of things [API.Mount] accepts. It is sealed — only
// *Resource[T] satisfies it (the unexported method) — so the non-generic API
// can collect heterogeneous resources without Go's (absent) generic methods.
type Mountable interface {
	http.Handler
	contributeOpenAPI(path string, doc *openapi3.T, gen *openapi3gen.Generator) error
}

// API collects mounted resources and emits a single OpenAPI 3.0 document for
// them. Build it with [NewAPI], add resources with [API.Mount] (or
// [API.Record]), and serve the spec with [API.SpecHandler].
type API struct {
	info    APIInfo
	entries []apiEntry

	securitySchemes map[string]*openapi3.SecurityScheme
	globalSecurity  []string

	once sync.Once
	body []byte
	err  error
}

type apiEntry struct {
	path string
	res  Mountable
}

// NewAPI starts an OpenAPI collector with the given document metadata.
func NewAPI(info APIInfo) *API { return &API{info: info} }

// Mount registers the resource on r at path and records it for the spec, so the
// path is named once. It is r.Mount plus bookkeeping.
func (api *API) Mount(r chi.Router, path string, rs Mountable) {
	r.Mount(path, rs)
	api.Record(path, rs)
}

// Record adds a resource to the spec without mounting it — for resources
// mounted via [Resource.Routes] with custom sibling actions.
func (api *API) Record(path string, rs Mountable) {
	api.entries = append(api.entries, apiEntry{path: path, res: rs})
}

// Spec builds the OpenAPI 3.0 document for the recorded resources.
func (api *API) Spec() (*openapi3.T, error) {
	doc := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       api.info.Title,
			Version:     api.info.Version,
			Description: api.info.Description,
		},
		Paths:      openapi3.NewPaths(),
		Components: &openapi3.Components{Schemas: openapi3.Schemas{}},
	}
	if api.info.BaseURL != "" {
		doc.Servers = openapi3.Servers{{URL: api.info.BaseURL}}
	}

	gen := openapi3gen.NewGenerator(
		openapi3gen.SchemaCustomizer(validateCustomizer),
		openapi3gen.CreateComponentSchemas(openapi3gen.ExportComponentSchemasOptions{
			ExportComponentSchemas: true,
			ExportTopLevelSchema:   true,
		}),
	)
	// Reflect the error envelope from the same struct handlers emit, so the
	// spec can't drift from the wire format.
	if _, err := gen.NewSchemaRefForValue(errorEnvelope{}, doc.Components.Schemas); err != nil {
		return nil, err
	}
	for _, e := range api.entries {
		if err := e.res.contributeOpenAPI(e.path, doc, gen); err != nil {
			return nil, err
		}
	}
	api.applySecurity(doc)
	// Manually-built $refs carry only their Ref string; resolve them against
	// the components so each SchemaRef.Value is populated (validation and
	// downstream tooling expect resolved refs).
	if err := openapi3.NewLoader().ResolveRefsIn(doc, nil); err != nil {
		return nil, err
	}
	return doc, nil
}

// SpecHandler serves the spec as JSON. The document is built once on first use
// and cached.
func (api *API) SpecHandler() burrow.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) error {
		api.once.Do(func() {
			doc, err := api.Spec()
			if err != nil {
				api.err = err
				return
			}
			api.body, api.err = doc.MarshalJSON()
		})
		if api.err != nil {
			return api.err
		}
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write(api.body)
		return err
	}
}

// --- per-resource emission ---

// errorSchemaName is the component name the error envelope reflects to (its Go
// type name in handlers.go).
const errorSchemaName = "errorEnvelope"

// contributeOpenAPI adds this resource's operations to doc, reflecting T (and
// any write-model DTOs) into component schemas via gen.
func (rs *Resource[T]) contributeOpenAPI(path string, doc *openapi3.T, gen *openapi3gen.Generator) error {
	schemas := doc.Components.Schemas
	// Pass a struct value (not *T): the generator only builds a component for a
	// struct at the top level, and inlines a pointer.
	tRef, err := gen.NewSchemaRefForValue(*new(T), schemas)
	if err != nil {
		return err
	}
	applyRequired(componentSchema(tRef, schemas), reflect.TypeFor[T]())

	createRef, err := rs.bodyRef(rs.createType, tRef, gen, schemas)
	if err != nil {
		return err
	}
	updateRef, err := rs.bodyRef(rs.updateType, tRef, gen, schemas)
	if err != nil {
		return err
	}
	pageRef, err := gen.NewSchemaRefForValue(burrow.PageResult{}, schemas)
	if err != nil {
		return err
	}

	// A presenter returns an arbitrary shape, so the response schema can't be
	// reflected from T — document a free-form object instead.
	respRef := tRef
	if rs.present != nil {
		respRef = openapi3.NewSchemaRef("", openapi3.NewObjectSchema())
	}

	tag := rs.tag(path)
	rs.registerTag(doc, tag)
	errResp := errResponse()
	coll := &openapi3.PathItem{}
	item := &openapi3.PathItem{}

	if rs.enabled[ActionList] {
		op := rs.buildOp(tag, ActionList, "list_"+tag, "List "+tag)
		rs.addListParams(op)
		op.AddResponse(http.StatusOK, jsonResponse("A page of "+tag, listSchemaRef(respRef, pageRef)))
		coll.Get = op
	}
	if rs.enabled[ActionCreate] {
		op := rs.buildOp(tag, ActionCreate, "create_"+tag, "Create "+tag)
		op.RequestBody = jsonBody(createRef)
		op.AddResponse(http.StatusCreated, jsonResponse("The created resource.", respRef))
		op.AddResponse(http.StatusBadRequest, errResp)
		coll.Post = op
	}
	if rs.enabled[ActionGet] {
		op := rs.buildOp(tag, ActionGet, "get_"+tag, "Get "+tag)
		op.AddParameter(idParam())
		op.AddResponse(http.StatusOK, jsonResponse("The requested resource.", respRef))
		op.AddResponse(http.StatusNotFound, errResp)
		item.Get = op
	}
	if rs.enabled[ActionUpdate] {
		op := rs.buildOp(tag, ActionUpdate, "update_"+tag, "Update "+tag+" (partial merge)")
		op.AddParameter(idParam())
		op.RequestBody = jsonBody(updateRef)
		op.AddResponse(http.StatusOK, jsonResponse("The updated resource.", respRef))
		op.AddResponse(http.StatusBadRequest, errResp)
		op.AddResponse(http.StatusNotFound, errResp)
		rs.addConcurrencyResponses(op, errResp)
		item.Patch = op
	}
	if rs.enabled[ActionReplace] {
		op := rs.buildOp(tag, ActionReplace, "replace_"+tag, "Replace "+tag+" (full)")
		op.AddParameter(idParam())
		op.RequestBody = jsonBody(createRef)
		op.AddResponse(http.StatusOK, jsonResponse("The replaced resource.", respRef))
		op.AddResponse(http.StatusBadRequest, errResp)
		op.AddResponse(http.StatusNotFound, errResp)
		rs.addConcurrencyResponses(op, errResp)
		item.Put = op
	}
	if rs.enabled[ActionDelete] {
		op := rs.buildOp(tag, ActionDelete, "delete_"+tag, "Delete "+tag)
		op.AddParameter(idParam())
		op.AddResponse(http.StatusNoContent, noContentResponse())
		op.AddResponse(http.StatusNotFound, errResp)
		item.Delete = op
	}

	if hasOperation(coll) {
		doc.Paths.Set(path, coll)
	}
	if hasOperation(item) {
		doc.Paths.Set(path+"/{id}", item)
	}
	return nil
}

// addListParams declares the list endpoint's query parameters from the
// resource's configured pagination and client-driven query options.
func (rs *Resource[T]) addListParams(op *openapi3.Operation) {
	if rs.cursor {
		op.AddParameter(queryParam("after", "Cursor for the next page", openapi3.NewStringSchema()))
	} else {
		op.AddParameter(queryParam("page", "1-based page number", openapi3.NewIntegerSchema()))
	}
	op.AddParameter(queryParam("limit", "Items per page", openapi3.NewIntegerSchema()))
	for field := range rs.filterable {
		op.AddParameter(queryParam(field, "Filter by "+field, openapi3.NewStringSchema()))
	}
	if len(rs.orderable) > 0 {
		op.AddParameter(queryParam("ordering", "Comma-separated sort fields ('-' = descending)", openapi3.NewStringSchema()))
	}
	if rs.fts || len(rs.searchFields) > 0 {
		op.AddParameter(queryParam("search", "Full-text/substring search term", openapi3.NewStringSchema()))
	}
	if len(rs.expandable) > 0 {
		op.AddParameter(queryParam("expand", "Comma-separated relations to inline", openapi3.NewStringSchema()))
	}
}

// addConcurrencyResponses adds the 428/412 responses for optimistic-concurrency
// resources.
func (rs *Resource[T]) addConcurrencyResponses(op *openapi3.Operation, errResp *openapi3.Response) {
	if !rs.concurrency {
		return
	}
	op.AddResponse(http.StatusPreconditionRequired, errResp)
	op.AddResponse(http.StatusPreconditionFailed, errResp)
}

// bodyRef returns the schema ref for a request body: the DTO type when set,
// otherwise the document type T.
func (rs *Resource[T]) bodyRef(dto reflect.Type, tRef *openapi3.SchemaRef, gen *openapi3gen.Generator, schemas openapi3.Schemas) (*openapi3.SchemaRef, error) {
	if dto == nil {
		return tRef, nil
	}
	ref, err := gen.NewSchemaRefForValue(reflect.New(dto).Elem().Interface(), schemas)
	if err != nil {
		return nil, err
	}
	applyRequired(componentSchema(ref, schemas), dto)
	return ref, nil
}

// --- openapi3 construction helpers ---

func newOp(tag, id, summary string) *openapi3.Operation {
	op := openapi3.NewOperation()
	op.Tags = []string{tag}
	op.OperationID = id
	op.Summary = summary
	return op
}

// tag returns the resource's OpenAPI tag: the WithTag name, or the path-derived
// default.
func (rs *Resource[T]) tag(path string) string {
	if rs.tagName != "" {
		return rs.tagName
	}
	return opID(path)
}

// registerTag adds a doc-level tag entry with prose when WithTag set a
// description, deduplicating by name.
func (rs *Resource[T]) registerTag(doc *openapi3.T, name string) {
	if rs.tagDesc == "" {
		return
	}
	for _, t := range doc.Tags {
		if t.Name == name {
			return
		}
	}
	doc.Tags = append(doc.Tags, &openapi3.Tag{Name: name, Description: rs.tagDesc})
}

// buildOp creates an operation and applies the resource's documentation
// overrides (WithActionDoc prose, WithSecurity requirement) for the action.
func (rs *Resource[T]) buildOp(tag, action, id, summary string) *openapi3.Operation {
	op := newOp(tag, id, summary)
	if d, ok := rs.docs[action]; ok {
		if d.summary != "" {
			op.Summary = d.summary
		}
		op.Description = d.description
	}
	if rs.security != nil {
		reqs := securityRequirements(*rs.security)
		op.Security = &reqs
	}
	return op
}

func idParam() *openapi3.Parameter {
	return openapi3.NewPathParameter("id").WithSchema(openapi3.NewStringSchema())
}

func queryParam(name, desc string, schema *openapi3.Schema) *openapi3.Parameter {
	return openapi3.NewQueryParameter(name).WithDescription(desc).WithSchema(schema)
}

func jsonBody(ref *openapi3.SchemaRef) *openapi3.RequestBodyRef {
	return &openapi3.RequestBodyRef{Value: openapi3.NewRequestBody().WithRequired(true).WithJSONSchemaRef(ref)}
}

func jsonResponse(desc string, ref *openapi3.SchemaRef) *openapi3.Response {
	return openapi3.NewResponse().WithDescription(desc).WithJSONSchemaRef(ref)
}

func noContentResponse() *openapi3.Response {
	return openapi3.NewResponse().WithDescription("No content")
}

// errResponse returns a JSON error-envelope response referencing the shared
// component.
func errResponse() *openapi3.Response {
	ref := openapi3.NewSchemaRef("#/components/schemas/"+errorSchemaName, nil)
	return openapi3.NewResponse().WithDescription("Error").WithJSONSchemaRef(ref)
}

// listSchemaRef wraps the item schema in the PageResponse envelope. items needs
// the per-resource item ref spliced in, so it is hand-wired; the pagination
// half reuses the schema reflected from burrow.PageResult.
func listSchemaRef(itemsRef, pageRef *openapi3.SchemaRef) *openapi3.SchemaRef {
	items := openapi3.NewArraySchema()
	items.Items = itemsRef

	wrapper := openapi3.NewObjectSchema()
	wrapper.WithPropertyRef("items", openapi3.NewSchemaRef("", items))
	wrapper.WithPropertyRef("pagination", pageRef)
	return openapi3.NewSchemaRef("", wrapper)
}

func hasOperation(p *openapi3.PathItem) bool {
	return p.Get != nil || p.Post != nil || p.Patch != nil || p.Put != nil || p.Delete != nil
}

// componentSchema resolves the concrete schema for a generator-produced ref,
// whether it is an inline value or a $ref into the components map.
func componentSchema(ref *openapi3.SchemaRef, schemas openapi3.Schemas) *openapi3.Schema {
	if ref == nil {
		return nil
	}
	if ref.Value != nil {
		return ref.Value
	}
	name := strings.TrimPrefix(ref.Ref, "#/components/schemas/")
	if s := schemas[name]; s != nil {
		return s.Value
	}
	return nil
}

// applyRequired sets the object schema's required list from `validate:"required"`
// tags on rt's fields (recursing embedded structs).
func applyRequired(schema *openapi3.Schema, rt reflect.Type) {
	if schema == nil {
		return
	}
	for rt.Kind() == reflect.Pointer {
		rt = rt.Elem()
	}
	if rt.Kind() != reflect.Struct {
		return
	}
	for f := range rt.Fields() {
		name, _, _ := strings.Cut(f.Tag.Get("json"), ",")
		if f.Anonymous && name == "" {
			applyRequired(schema, f.Type)
			continue
		}
		if name == "" || name == "-" {
			continue
		}
		if validateHas(f.Tag.Get("validate"), "required") && !slices.Contains(schema.Required, name) {
			schema.Required = append(schema.Required, name)
		}
	}
}

// validateCustomizer maps go-playground/validator field rules onto schema
// constraints (kin-openapi has no native bridge). Object-level `required` is
// handled separately by applyRequired.
func validateCustomizer(_ string, _ reflect.Type, tag reflect.StructTag, schema *openapi3.Schema) error {
	rules := tag.Get("validate")
	if rules == "" {
		return nil
	}
	isString := schema.Type != nil && schema.Type.Is("string")
	for rule := range strings.SplitSeq(rules, ",") {
		key, val, _ := strings.Cut(rule, "=")
		switch key {
		case "email":
			schema.Format = "email"
		case "url", "uri":
			schema.Format = "uri"
		case "uuid":
			schema.Format = "uuid"
		case "oneof":
			for v := range strings.FieldsSeq(val) {
				schema.Enum = append(schema.Enum, v)
			}
		case "min":
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				if isString {
					schema.MinLength = uint64(n)
				} else {
					schema.Min = &n
				}
			}
		case "max":
			if n, err := strconv.ParseFloat(val, 64); err == nil {
				if isString {
					m := uint64(n)
					schema.MaxLength = &m
				} else {
					schema.Max = &n
				}
			}
		}
	}
	return nil
}

func validateHas(rules, want string) bool {
	for rule := range strings.SplitSeq(rules, ",") {
		if key, _, _ := strings.Cut(rule, "="); key == want {
			return true
		}
	}
	return false
}

// opID turns a mount path into an operation-id / tag fragment.
func opID(path string) string {
	id := strings.Trim(path, "/")
	id = strings.ReplaceAll(id, "/", "_")
	if id == "" {
		return "root"
	}
	return id
}
