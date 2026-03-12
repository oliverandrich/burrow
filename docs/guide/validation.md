# Validation

Burrow integrates [go-playground/validator](https://github.com/go-playground/validator) for struct validation. When you call `Bind()`, the request body is decoded **and** validated in one step. You can also call `Validate()` directly on any struct.

!!! tip "For HTML forms, use the forms package"
    If you're building HTML forms with model binding and template rendering, the [forms package](forms.md) handles binding, validation, and error display in one step. This guide covers the lower-level `Bind()`/`Validate()` API for JSON APIs and standalone validation.

## Struct Tags

Burrow uses two separate struct tag types for request handling:

- **`form`** (or `json`) — controls how `Bind()` maps request fields to struct fields. This is the *binding* tag.
- **`validate`** — controls which validation rules `Bind()` / `Validate()` applies. This is the *validation* tag.

The two work together but serve different purposes:

```go
var req struct {
    Name  string `form:"name"  validate:"required"`
    Email string `form:"email" validate:"required,email"`
    Age   int    `form:"age"   validate:"gte=0,lte=130"`
}
if err := burrow.Bind(r, &req); err != nil {
    // handle validation error
}
```

`form:"name"` tells `Bind()` to read the value from the `name` form field. `validate:"required"` tells the validator to reject empty values. A field can have one, both, or neither tag.

## Validation Tags

Common tags:

| Tag | Example | Description |
|-----|---------|-------------|
| `required` | `validate:"required"` | Field must not be empty/zero |
| `email` | `validate:"email"` | Must be a valid email address |
| `min` | `validate:"min=3"` | Minimum length (string) or value (number) |
| `max` | `validate:"max=100"` | Maximum length (string) or value (number) |
| `len` | `validate:"len=8"` | Exact length |
| `gte` | `validate:"gte=0"` | Greater than or equal to |
| `lte` | `validate:"lte=130"` | Less than or equal to |
| `url` | `validate:"url"` | Must be a valid URL |

Tags can be combined with commas: `validate:"required,email"`.

For the full list of available tags, see the [validator documentation](https://pkg.go.dev/github.com/go-playground/validator/v10#hdr-Baked_In_Validators_and_Tags).

## Standalone Validation

Use `Validate()` when you need to validate a struct without binding from a request:

```go
user := User{Name: "", Email: "invalid"}
if err := burrow.Validate(user); err != nil {
    // handle validation error
}
```

`Validate()` returns `nil` for non-structs or structs without `validate` tags.

## Handling Errors

Both `Bind()` and `Validate()` return a `*ValidationError` when validation fails. Use `errors.As` to extract it:

```go
if err := burrow.Bind(r, &req); err != nil {
    var ve *burrow.ValidationError
    if errors.As(err, &ve) {
        // ve.Errors contains per-field errors
        // handle validation failure (return JSON error, re-render form, etc.)
    }
    return err
}
```

!!! warning "ValidationError is not an HTTPError"
    `ValidationError` is **not** automatically converted to a 400 response by `Handle()`. Your handler must check for it explicitly and render an appropriate response.

### ValidationError

```go
type ValidationError struct {
    Errors []FieldError
}
```

`Error()` returns a summary string like `"validation failed: name is required; email is required"`.

### FieldError

Each `FieldError` describes one failed validation:

| Field | Type | Description |
|-------|------|-------------|
| `Field` | `string` | Field name (from `form` tag, `json` tag, or Go field name) |
| `Tag` | `string` | Validation tag that failed (e.g., `"required"`, `"email"`) |
| `Param` | `string` | Tag parameter (e.g., `"3"` for `min=3`), empty for parameterless tags |
| `Value` | `any` | The value that failed validation |
| `Message` | `string` | Human-readable error message in English |

### HasField

Check whether a specific field has a validation error:

```go
if ve.HasField("email") {
    // highlight the email input
}
```

## Field Name Resolution

Field names in `FieldError.Field` are resolved in this order:

1. `form` struct tag (if present)
2. `json` struct tag (if present)
3. Go field name

```go
type Request struct {
    Email string `form:"email_address" json:"email"`  // Field = "email_address"
    Name  string `json:"full_name"`                    // Field = "full_name"
    Age   int                                          // Field = "Age"
}
```

## Translating Error Messages

When using the [i18n](i18n.md#translating-validation-errors) package, validation messages can be translated to the user's locale:

```go
var ve *burrow.ValidationError
if errors.As(err, &ve) {
    ve.Translate(r.Context(), i18n.TData)
    // ve.Errors[*].Message now contains translated messages
}
```
