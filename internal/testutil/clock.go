package testutil

import (
	"sync"
	"time"
)

// FakeClock is a deterministic, concurrency-safe clock for black-box tests.
type FakeClock struct {
	mu  sync.Mutex
	now time.Time
}

func NewFakeClock(start time.Time) *FakeClock {
	return &FakeClock{now: start.UTC()}
}

func (c *FakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *FakeClock) Set(next time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = next.UTC()
}

func (c *FakeClock) Advance(delta time.Duration) time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(delta).UTC()
	return c.now
}
