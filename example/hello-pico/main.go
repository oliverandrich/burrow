// Command hello-pico is a minimal burrow application that serves a single
// "Hello, World!" page styled with PicoCSS. It mirrors example/hello and
// exists so the maintainer can live with the contrib/pico app in real use
// before deciding whether to migrate further apps to it.
//
// Run it with:
//
//	go run ./example/hello-pico
//
// Then open http://localhost:8080 in your browser.
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
	"github.com/oliverandrich/burrow/contrib/pico"
	"github.com/oliverandrich/burrow/contrib/staticfiles"
	_ "github.com/oliverandrich/den/backend/sqlite"
	"github.com/urfave/cli/v3"
)

//go:embed templates
var templateFS embed.FS

//go:embed translations
var translationFS embed.FS

var emptyFS embed.FS

func main() {
	staticApp, err := staticfiles.New(emptyFS)
	if err != nil {
		log.Fatal(err)
	}

	hello := &helloApp{}

	srv := burrow.NewServer(
		staticApp,
		htmx.New(),
		pico.New(pico.WithCompactType()),
		hello,
	)
	srv.SetLayout(pico.NavLayout())

	cmd := &cli.Command{
		Name:   "hello-pico",
		Usage:  "Minimal burrow hello world application styled with PicoCSS",
		Flags:  srv.Flags(nil),
		Action: srv.Run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

type helloApp struct{}

func (a *helloApp) Name() string         { return "hello" }
func (a *helloApp) TranslationFS() fs.FS { return translationFS }

func (a *helloApp) TemplateFS() fs.FS {
	sub, _ := fs.Sub(templateFS, "templates")
	return sub
}

func (a *helloApp) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.Render(w, r, http.StatusOK, "hello/home", map[string]any{
			"Title": "Hello",
		})
	}))
}
