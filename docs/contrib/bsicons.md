# Bootstrap Icons

Inline SVG icons from [Bootstrap Icons](https://icons.getbootstrap.com/) as Go functions returning `template.HTML`. Only icons actually used in your code end up in the compiled binary.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/bsicons`

## Usage

Each icon is a Go function that returns `template.HTML` containing an inline SVG:

```go
import "codeberg.org/oliverandrich/burrow/contrib/bsicons"

html := bsicons.Trash()
html := bsicons.House()
html := bsicons.PersonCircle()
```

### CSS Classes

Pass optional CSS classes to the icon:

```go
// Single class string
html := bsicons.JournalText("fs-1 d-block mb-2")

// Multiple arguments are joined with spaces
html := bsicons.People("fs-1", "text-primary")
```

### In Templates via FuncMap

Apps that need icons in templates register them via `HasFuncMap`. For example, the admin app registers icon functions like `iconGearFill`:

```html
{{ define "admin/sidebar" -}}
<nav>
  {{ iconGearFill "me-2" }} Settings
</nav>
{{- end }}
```

### NavItems

Use icons in navigation items:

```go
func (a *App) NavItems() []burrow.NavItem {
    return []burrow.NavItem{
        {
            Label:    "Notes",
            URL:      "/notes",
            Icon:     bsicons.JournalText(),
            Position: 20,
        },
    }
}
```

The `NavItem.Icon` field is `template.HTML`, so the SVG is rendered directly in layout templates via `{{ .Icon }}`.

### In Templates with Spacing

When an icon needs spacing from adjacent text in an HTML template:

```html
<button class="btn btn-primary">
    <span class="me-1">{{ iconPlusLg }}</span>Add Note
</button>
```

## How It Works

Each icon renders an inline `<svg>` element:

```html
<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em"
     fill="currentColor" style="vertical-align:-.125em"
     viewBox="0 0 16 16">
  <path d="..."/>
</svg>
```

Key properties:

- **`width="1em" height="1em"`** — scales with parent font-size (Bootstrap `fs-1`, `fs-4` etc.)
- **`fill="currentColor"`** — inherits text colour (Bootstrap `text-primary`, `text-danger` etc.)
- **`vertical-align:-.125em`** — aligns with text baseline

## Updating Icons

To update to a new Bootstrap Icons release:

```bash
just update-icons 1.14.0
```

This downloads the release, extracts all SVGs, and regenerates the Go source file. See `contrib/bsicons/internal/generate/` for the generator source.

## Available Icons

All ~2000 Bootstrap Icons are available. Function names are PascalCase versions of the icon names:

| Icon name | Function |
|-----------|----------|
| `house` | `bsicons.House()` |
| `person-circle` | `bsicons.PersonCircle()` |
| `box-arrow-right` | `bsicons.BoxArrowRight()` |
| `journal-text` | `bsicons.JournalText()` |
| `0-circle` | `bsicons.Nr0Circle()` |

Names starting with a digit are prefixed with `Nr`. Browse all icons at [icons.getbootstrap.com](https://icons.getbootstrap.com/).
