package mirageecs

import (
	"sync"
	"time"
)

// accessCount is a map for access count
// key is a time truncated by accessCounter.unit
type accessCount map[time.Time]int64

// accessCounter is a thread-safe counter for access
type AccessCounter struct {
	mu    *sync.Mutex
	unit  time.Duration
	count accessCount
}

// NewAccessCounter returns a new access counter
// unit is the time unit for the counter (default: time.Minute)
func NewAccessCounter(unit time.Duration) *AccessCounter {
	if unit == 0 {
		unit = time.Minute
	}
	c := &AccessCounter{
		mu:    new(sync.Mutex),
		count: make(accessCount, 2), // 2 is enough for most cases
		unit:  unit,
	}
	c.fill()
	return c
}

// Add increments the access counter
func (c *AccessCounter) Add() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().Truncate(c.unit)
	c.count[now]++
}

// Collect returns the access count and resets the counter
func (c *AccessCounter) Collect() accessCount {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := make(accessCount, len(c.count))
	for k, v := range c.count {
		r[k] = v
		delete(c.count, k)
	}
	c.fill()
	return r
}

func (c *AccessCounter) fill() {
	c.count[time.Now().Truncate(c.unit)] = 0
}
