// Command step02 adds database models and migrations to the polls app.
package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"codeberg.org/oliverandrich/burrow"
	"github.com/go-chi/chi/v5"
	"github.com/urfave/cli/v3"

	"tutorial/step02/internal/polls"
)

func main() {
	srv := burrow.NewServer(
		&homepageApp{},
		polls.New(),
	)

	cmd := &cli.Command{
		Name:    "polls",
		Usage:   "Polls tutorial application",
		Version: "0.2.0",
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
		return burrow.Text(w, http.StatusOK, "Hello, Polls! Visit /polls to see the polls.")
	}))
}
