// Command step01 is the first step of the burrow tutorial.
// It creates a minimal server with a homepage that says "Hello, Polls!".
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"
)

func main() {
	srv := burrow.NewServer(
		&homepageApp{},
	)

	cmd := &cli.Command{
		Name:    "polls",
		Usage:   "Polls tutorial application",
		Version: "0.1.0",
		Flags:   srv.Flags(nil),
		Action:  srv.Run,
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}

// homepageApp serves a simple homepage at /.
type homepageApp struct{}

func (a *homepageApp) Name() string                       { return "homepage" }
func (a *homepageApp) Register(_ *burrow.AppConfig) error { return nil }
func (a *homepageApp) Routes(r chi.Router) {
	r.Get("/", burrow.Handle(func(w http.ResponseWriter, r *http.Request) error {
		return burrow.Text(w, http.StatusOK, "Hello, Polls!")
	}))
}
