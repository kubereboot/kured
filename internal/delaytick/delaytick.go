package delaytick

import (
	"math/rand"
	"time"
)

// New ticks regularly after an initial delay randomly distributed between d/2 and d + d/2
func New(s rand.Source, d time.Duration) <-chan time.Time {
	c := make(chan time.Time)

	go func() {
		random := rand.New(s)
		time.Sleep(time.Duration(float64(d)/2 + float64(d)*random.Float64()))
		c <- time.Now()
		for t := range time.Tick(d) {
			c <- t
		}
	}()

	return c
}
