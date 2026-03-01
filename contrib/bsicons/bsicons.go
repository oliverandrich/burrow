// Package bsicons provides all Bootstrap Icons as inline SVG templ.Components.
//
// Each icon is available as a named function (e.g. bsicons.Trash(), bsicons.House())
// that returns a templ.Component rendering an inline <svg> element. Icons scale with
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
	"context"
	"io"
	"strings"

	"github.com/a-h/templ"
)

func icon(svgContent string, class ...string) templ.Component {
	return templ.ComponentFunc(func(_ context.Context, w io.Writer) error {
		var b strings.Builder
		b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" width="1em" height="1em" fill="currentColor" style="vertical-align:-.125em" viewBox="0 0 16 16"`)
		if len(class) > 0 {
			if cls := strings.Join(class, " "); cls != "" {
				b.WriteString(` class="`)
				b.WriteString(templ.EscapeString(cls))
				b.WriteByte('"')
			}
		}
		b.WriteByte('>')
		b.WriteString(svgContent)
		b.WriteString(`</svg>`)
		_, err := io.WriteString(w, b.String())
		return err
	})
}
