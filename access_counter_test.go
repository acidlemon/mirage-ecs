package main

import (
	"testing"
	"time"
)

func TestAccessCounter(t *testing.T) {
	c := newAccessCounter(time.Second)
	start := time.Now().Truncate(time.Second)
	c.Add()
	c.Add()
	c.Add()
	time.Sleep(time.Second)
	c.Add()
	c.Add()
	c.Add()
	c.Add()
	c.Add()
	r := c.Collect()
	if len(r) != 2 {
		t.Errorf("could not collect access count %#v", r)
	}
	if r[start] != 3 {
		t.Errorf("could not collect access count %#v", r)
	}
	if r[start.Add(time.Second)] != 5 {
		t.Errorf("could not collect access count %#v", r)
	}
	r2 := c.Collect()
	if len(r2) != 0 {
		t.Errorf("counter should be empty %#v", r2)
	}
}
