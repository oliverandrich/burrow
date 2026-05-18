package main

import (
	"context"

	"github.com/oliverandrich/burrow/internal/tailwind"
	"github.com/urfave/cli/v3"
)

func tailwindCommand() *cli.Command {
	return &cli.Command{
		Name:            "tailwind",
		Usage:           "Run tailwindcss with auto-discovered @source listing",
		ArgsUsage:       "[tailwindcss-args...]",
		Description:     "All arguments after `tailwind` are forwarded verbatim to the tailwindcss CLI. A .tailwind/sources.css listing is regenerated on every invocation, pointing at every contrib/<app>/templates/ and internal/<app>/templates/ directory in scope.",
		SkipFlagParsing: true,
		Action: func(ctx context.Context, cmd *cli.Command) error {
			return tailwind.Run(ctx, cmd.Args().Slice())
		},
	}
}
