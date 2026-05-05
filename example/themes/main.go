// Command themes is a Burrow demo app that previews every µCSS accent
// variant against Burrow's muted semantic palette and against µCSS's
// upstream defaults. Inspired by https://mucss.org/themes — same layout
// (Buttons / Card / Alerts / Form / Badges / Progress) plus a switcher
// row at the top for accent color, palette mode, and light/dark theme.
//
// Run it with:
//
//	go run ./example/themes
//
// Then open http://localhost:8080.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/oliverandrich/burrow"
	"github.com/oliverandrich/burrow/contrib/htmx"
	"github.com/oliverandrich/burrow/contrib/mucss"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	_ "github.com/oliverandrich/den/backend/sqlite" // register sqlite:// scheme
	"github.com/urfave/cli/v3"
)

//go:embed templates
var templateFS embed.FS

var emptyFS embed.FS

func main() {
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	srv := burrow.NewServer(
		staticApp,
		htmx.New(),
		mucss.New(),
		&themesApp{},
	)
	srv.SetLayout("themes/layout")

	cmd := &cli.Command{
		Name:   "themes",
		Usage:  "Burrow µCSS theme preview",
		Flags:  srv.Flags(nil),
		Action: srv.Run,
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

type themesApp struct{}

func (a *themesApp) Name() string { return "themes" }

func (a *themesApp) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

// accent normalises a [mucss.Color] for template use: [mucss.Default]
// becomes Slug="default" / Label="µCSS default" so templates can drop
// the `eq (printf "%v" .) ""` ladder.
type accent struct{ Slug, Label string }

func accents() []accent {
	colors := mucss.AllColors()
	out := make([]accent, 0, len(colors))
	for _, c := range colors {
		if c == mucss.Default {
			out = append(out, accent{Slug: "default", Label: "µCSS default"})
			continue
		}
		out = append(out, accent{Slug: string(c), Label: string(c)})
	}
	return out
}

func (a *themesApp) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.Render(w, r, http.StatusOK, "themes/preview", map[string]any{
			"Title":   "µCSS Theme Preview",
			"Accents": accents(),
		})
	}))
}
