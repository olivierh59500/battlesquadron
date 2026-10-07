package engine

import "testing"

// FS-UAE cycle snapshots prove one terrain step per two PAL fields, while
// the ship and its invulnerability timer update on every field.
func TestOriginalPALAndWorldCadences(t *testing.T) {
	e := newFixture(t)
	e.Data.Stages = []Stage{{Height: 8192}}
	e.Players[0].Invulnerable = 100
	x := e.Players[0].X
	for range 40 {
		e.Tick([2]Input{{X: 1}})
	}
	if e.Frame != 40 || e.Scroll != 20 || e.Players[0].X != x+80 || e.Players[0].Invulnerable != 60 {
		t.Fatalf("original physical rates: frame=%d scroll=%d ship=%+v", e.Frame, e.Scroll, e.Players[0])
	}
}

func TestOriginalJoystickSampleAndSecondFieldLatch(t *testing.T) {
	e := newFixture(t)
	x := e.Players[0].X
	e.Tick([2]Input{{X: 1}})
	e.Tick([2]Input{{X: -1}})
	if e.Players[0].X != x+4 {
		t.Fatal("the second field did not preserve the first joystick sample")
	}
	e.Tick([2]Input{{X: -1}})
	if e.Players[0].X != x+2 {
		t.Fatal("the next first field did not refresh the joystick")
	}
}
