package replay

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

func TestOrdinaryInputsRoundTripAndCounts(t *testing.T) {
	r := Recording{}
	for frame := 0; frame < 300; frame++ {
		r.Append([2]engine.Input{{X: frame/100 - 1, Y: -1, Fire: frame%10 < 5, Nova: frame == 42}, {X: 1}})
	}
	var encoded bytes.Buffer
	if err := Encode(&encoded, r); err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(r, decoded) {
		t.Fatal("input sequence changed during encoding")
	}
	if len(r.Runs) >= 300 {
		t.Fatal("held joystick states were not compressed")
	}
}

func TestRejectTruncatedRecording(t *testing.T) {
	if _, err := Decode(bytes.NewReader([]byte(magic))); err == nil {
		t.Fatal("truncated source fingerprint accepted")
	}
}
