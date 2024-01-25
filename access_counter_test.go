package mirageecs_test

import (
	"testing"
	"time"

	mirageecs "github.com/acidlemon/mirage-ecs/v2"
)

func TestAccessCounter(t *testing.T) {
	c := mirageecs.NewAccessCounter(time.Second)
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
	for _, v := range r2 {
		if v != 0 {
			t.Errorf("counter should be zero %#v", r2)
		}
	}
}
