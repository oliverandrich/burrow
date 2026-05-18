package main

import (
	"context"
	"os/exec"
	"runtime/debug"
	"strings"
)

const burrowModulePath = "github.com/oliverandrich/burrow"

// readBurrowVersion returns the version of the github.com/oliverandrich/burrow
// module this binary was built from.
//
// Resolution order:
//  1. `runtime/debug.ReadBuildInfo` — `Main.Version` for an installed binary
//     (`go install github.com/oliverandrich/burrow/cmd/burrow@vX.Y.Z`), or
//     `Deps[].Version` when burrow is consumed as a dependency.
//  2. `git describe --tags --abbrev=0 --match 'v*'` in the current working
//     directory — covers `go run ./cmd/burrow` from inside the burrow source
//     tree, which is the dev workflow.
//
// Returns the empty string when neither resolution yields a version.
func readBurrowVersion() string {
	if v := readVersionFromBuildInfo(); v != "" {
		return v
	}
	return readVersionFromGit()
}

func readVersionFromBuildInfo() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}
	if info.Main.Path == burrowModulePath && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	for _, dep := range info.Deps {
		if dep.Path == burrowModulePath && dep.Version != "" {
			return dep.Version
		}
	}
	return ""
}

func readVersionFromGit() string {
	out, err := exec.CommandContext(context.Background(), "git", "describe", "--tags", "--abbrev=0", "--match", "v*").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
