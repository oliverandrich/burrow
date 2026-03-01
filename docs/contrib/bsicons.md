# Bootstrap Icons

Inline SVG icons from [Bootstrap Icons](https://icons.getbootstrap.com/) as `templ.Component` functions. Only icons actually used in your code end up in the compiled binary.

**Package:** `codeberg.org/oliverandrich/burrow/contrib/bsicons`

## Usage

Each icon is a Go function that returns a `templ.Component`:

```go
import "codeberg.org/oliverandrich/burrow/contrib/bsicons"

// In a templ template:
@bsicons.Trash()
@bsicons.House()
@bsicons.PersonCircle()
```

### CSS Classes

Pass optional CSS classes to the icon:

```go
// Single class string
@bsicons.JournalText("fs-1 d-block mb-2")

// Multiple arguments are joined
@bsicons.People("fs-1", "text-primary")
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

### In Templates with Spacing

When an icon needs spacing from adjacent text, wrap it in a `<span>`:

```templ
<button class="btn btn-primary">
    <span class="me-1">@bsicons.PlusLg()</span>Add Note
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
- **`fill="currentColor"`** — inherits text color (Bootstrap `text-primary`, `text-danger` etc.)
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
