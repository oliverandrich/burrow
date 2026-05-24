//go:build windows

package selfupdate

// acquireFlock is a no-op on Windows: there is no portable flock
// equivalent we can pull from stdlib. Best-effort support only —
// burrow selfupdate is not tested on Windows.
func acquireFlock(_ string) (func(), error) {
	return func() {}, nil
}
