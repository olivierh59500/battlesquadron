package replay

import (
	"testing"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

func TestCursorOwnsInputsAndResetsAcrossRuns(t *testing.T) {
	want := [][2]engine.Input{{{X: 1, Fire: true}}, {{X: 1, Fire: true}}, {{Y: -1, Nova: true}, {X: -1}}}
	var recording Recording
	for _, inputs := range want {
		recording.Append(inputs)
	}
	cursor, err := NewCursor(recording)
	if err != nil {
		t.Fatal(err)
	}
	recording.Runs[0].Inputs = [2]engine.Input{}
	for range 2 {
		if cursor.Position() != 0 || cursor.Fields() != uint64(len(want)) {
			t.Fatal("cursor did not retain its length or reset position")
		}
		for index, expected := range want {
			actual, ok := cursor.Next()
			if !ok || actual != expected || cursor.Position() != uint64(index+1) {
				t.Fatalf("field %d: got %+v, available=%t", index, actual, ok)
			}
		}
		if inputs, ok := cursor.Next(); ok || inputs != [2]engine.Input{} {
			t.Fatal("exhausted cursor repeated an input")
		}
		cursor.Reset()
	}
	if allocations := testing.AllocsPerRun(100, func() {
		cursor.Reset()
		for range want {
			cursor.Next()
		}
	}); allocations != 0 {
		t.Fatalf("playback allocated %g objects", allocations)
	}
}

func TestCursorRejectsInvalidRuns(t *testing.T) {
	for _, recording := range []Recording{{Fields: 1}, {Fields: 1, Runs: []Run{{Fields: 0}}}, {Fields: 2, Runs: []Run{{Fields: 1}}}} {
		if _, err := NewCursor(recording); err == nil {
			t.Fatal("invalid recording accepted")
		}
	}
}

func TestExpertChecksumRejectsUnverifiedInputs(t *testing.T) {
	if _, err := DecodeExpert([]byte("unverified inputs")); err == nil {
		t.Fatal("unverified expert recording accepted")
	}
	proof := ExpertMetadata()
	if proof.Options != engine.DefaultOptions() || proof.Players != 1 || proof.Options.Invulnerable {
		t.Fatal("expert metadata does not describe an ordinary one-player start")
	}
}
