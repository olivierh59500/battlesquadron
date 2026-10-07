package controls

import "testing"

// A released stick must not steal a fire finger that is still down.
func TestIndependentPointers(t *testing.T) {
	c := New(480, 256)
	c.Update(map[int]Point{1: {30, 180}, 2: {435, 220}})
	s := c.Update(map[int]Point{1: {55, 155}, 2: {435, 220}, 3: {440, 140}})
	if s.X != 1 || s.Y != -1 || !s.Fire || !s.Nova {
		t.Fatalf("combined controls: %+v", s)
	}
	s = c.Update(map[int]Point{2: {435, 220}})
	if s.X != 0 || s.Y != 0 || !s.Fire || s.Nova {
		t.Fatalf("release: %+v", s)
	}
	c.Cancel()
	if s = c.Update(nil); s != (State{}) {
		t.Fatalf("cancelled input: %+v", s)
	}
}

func TestDeadZoneAndEightDirections(t *testing.T) {
	c := New(480, 256)
	c.Update(map[int]Point{0: {30, 180}})
	if s := c.Update(map[int]Point{0: {34, 182}}); s.X != 0 || s.Y != 0 {
		t.Fatal(s)
	}
	if s := c.Update(map[int]Point{0: {30, 145}}); s.X != 0 || s.Y != -1 {
		t.Fatal(s)
	}
	if s := c.Update(map[int]Point{0: {2, 190}}); s.X != -1 || s.Y != 0 {
		t.Fatal(s)
	}
}
