package ratelimit_test

import (
	"context"
	"fmt"
	"time"

	"github.com/oliverandrich/burrow/contrib/ratelimit"
)

func ExampleNewLimiter() {
	// Allow 1 request per second with a burst of 2.
	lim := ratelimit.NewLimiter(1, 2, time.Minute, 0)
	defer lim.Stop()

	// First two requests fit within the burst window.
	allowed1, _ := lim.Allow("client-1")
	allowed2, _ := lim.Allow("client-1")
	// Third request exceeds the burst.
	allowed3, _ := lim.Allow("client-1")

	fmt.Println(allowed1)
	fmt.Println(allowed2)
	fmt.Println(allowed3)
	// Output:
	// true
	// true
	// false
}

func ExampleWithRetryAfter() {
	ctx := ratelimit.WithRetryAfter(context.Background(), 5*time.Second)

	fmt.Println(ratelimit.RetryAfter(ctx))
	// Output:
	// 5s
}
