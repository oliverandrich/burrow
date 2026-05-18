// Command burrow-tailwind is a deprecation shim that delegates to
// `burrow tailwind`. It prints a one-line deprecation notice to stderr
// on every invocation and forwards all arguments to the new
// implementation in [github.com/oliverandrich/burrow/internal/tailwind].
//
// Scheduled for removal in v0.22. Migrate by replacing
//
//	go tool burrow-tailwind ...
//
// with
//
//	go tool burrow tailwind ...
//
// in your `.mise.toml`, `.air.toml`, and any other build scripts.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/oliverandrich/burrow/internal/tailwind"
)

func main() {
	fmt.Fprintln(os.Stderr, "burrow-tailwind: deprecated — use `burrow tailwind` (or `go tool burrow tailwind`) instead. This shim will be removed in v0.22.")

	if err := tailwind.Run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "burrow-tailwind:", err)
		os.Exit(1)
	}
}
