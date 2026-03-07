// Package bsicons provides all Bootstrap Icons as inline SVG template.HTML values.
//
// This is a standalone utility package — it does not implement [burrow.App]
// and requires no registration. Import it and call icon functions directly.
//
// Each icon is available as a named function (e.g. bsicons.Trash(), bsicons.House())
// that returns a template.HTML value containing an inline <svg> element. Icons scale with
// font-size (width/height="1em") and inherit text color (fill="currentColor").
//
// An optional class parameter can be passed to add CSS classes to the SVG element:
//
//	bsicons.Trash("fs-1 d-block mb-2")
//
// Only icons actually referenced in your code end up in the compiled binary —
// the Go linker eliminates unused icon functions automatically.
package bsicons

import (
	"html"
	"html/template"
	"strings"
)

func icon(svgContent string, class ...string) template.HTML {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" fill="currentColor" style="vertical-align:-.125em" viewBox="0 0 16 16"`)
	if len(class) > 0 {
		if cls := strings.Join(class, " "); cls != "" {
			b.WriteString(` class="`)
			b.WriteString(html.EscapeString(cls))
			b.WriteByte('"')
		}
	}
	b.WriteByte('>')
	b.WriteString(svgContent)
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) //nolint:gosec // SVG content is from generated code, class is escaped
}
