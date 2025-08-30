// Package delaytick provides utilities for scheduling periodic events
// with an initial randomized delay. It is primarily used to delay the
// start of regular ticks, helping to avoid thundering herd problems
// when multiple nodes begin operations simultaneously.
// You can use that a random ticker in other projects, but there is
// no garantee that it will stay (initial plan was to move it to internal)
package delaytick

import (
	"math/rand"
	"time"
)

// New ticks regularly after an initial delay randomly distributed between d/2 and d + d/2
func New(s rand.Source, d time.Duration) <-chan time.Time {
	c := make(chan time.Time)

	go func() {
		// #nosec G404 -- math/rand is used here for non-security timing jitter
		random := rand.New(s)
		time.Sleep(time.Duration(float64(d)/2 + float64(d)*random.Float64()))
		c <- time.Now()
		for t := range time.Tick(d) {
			c <- t
		}
	}()

	return c
}
