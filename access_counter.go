package mirageecs

import (
	"sync"
	"time"
)

// accessCounter is a thread-safe counter for access
type accessCounter struct {
	mu    *sync.Mutex
	unit  time.Duration
	count map[time.Time]int64
}

// newAccessCounter returns a new access counter
// unit is the time unit for the counter (default: time.Minute)
func newAccessCounter(unit time.Duration) *accessCounter {
	if unit == 0 {
		unit = time.Minute
	}
	c := &accessCounter{
		mu:    new(sync.Mutex),
		count: make(map[time.Time]int64, 2), // 2 is enough for most cases
		unit:  unit,
	}
	c.fill()
	return c
}

// Add increments the access counter
func (c *accessCounter) Add() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().Truncate(c.unit)
	c.count[now]++
}

// Collect returns the access count and resets the counter
func (c *accessCounter) Collect() map[time.Time]int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := make(map[time.Time]int64, len(c.count))
	for k, v := range c.count {
		r[k] = v
		delete(c.count, k)
	}
	c.fill()
	return r
}

func (c *accessCounter) fill() {
	c.count[time.Now().Truncate(c.unit)] = 0
}
