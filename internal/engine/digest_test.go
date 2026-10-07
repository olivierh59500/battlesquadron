package engine

import "testing"

func TestReplayDigestIncludesLatchedInputsDuringEntry(t *testing.T) {
	first, err := New(fixtureData())
	if err != nil {
		t.Fatal(err)
	}
	first.Start(1)
	second := first.Clone()
	first.Tick([2]Input{{X: -1}})
	second.Tick([2]Input{{X: 1}})
	// Entry motion ignores the joystick, while the next PAL field retains it.
	// A position-only replay check would miss this difference in hidden state.
	if first.Players != second.Players || first.Frame != second.Frame || first.Scroll != second.Scroll {
		t.Fatal("entry fixture did not retain identical visible state")
	}
	if first.Digest() == second.Digest() {
		t.Fatal("different latched joystick inputs produced the same replay digest")
	}
	if first.Digest() != first.Clone().Digest() {
		t.Fatal("an isolated copy changed the replay digest")
	}
}
