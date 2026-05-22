package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

// runCommand creates a cli.Command with CoreFlags and runs it with the given args,
// returning the parsed command. Used by Config tests to exercise flag parsing.
func runCommand(t *testing.T, args ...string) *cli.Command {
	t.Helper()
	cmd := &cli.Command{
		Name:   "test",
		Flags:  CoreFlags(nil),
		Action: func(_ context.Context, _ *cli.Command) error { return nil },
	}
	err := cmd.Run(t.Context(), append([]string{"test"}, args...))
	require.NoError(t, err)
	return cmd
}
