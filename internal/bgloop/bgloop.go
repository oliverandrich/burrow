// Package bgloop provides panic recovery for long-lived background goroutines.
//
// An unrecovered panic in any goroutine crashes the whole Go process, not just
// that goroutine. Long-lived background loops (ticker-driven cleanup, queue
// pollers) must therefore guard each iteration so a transient panic is logged
// and the loop survives instead of taking the server down.
//
// Use Recover deferred inside the body of a loop iteration:
//
//	for {
//		select {
//		case <-ctx.Done():
//			return
//		case <-ticker.C:
//			func() {
//				defer bgloop.Recover("auth.backgroundCleanup")
//				doWork(ctx)
//			}()
//		}
//	}
package bgloop

import (
	"log/slog"
	"runtime"
)

// Recover recovers a panic in a background loop iteration, logging it with a
// stack trace under the given loop name. It is a no-op when there is no panic.
// Call it deferred at the top of the loop-iteration body.
func Recover(name string) {
	r := recover()
	if r == nil {
		return
	}
	stack := make([]byte, 4096)
	n := runtime.Stack(stack, false)
	slog.Error("panic recovered in background loop", "loop", name, "panic", r, "stack", string(stack[:n]))
}
