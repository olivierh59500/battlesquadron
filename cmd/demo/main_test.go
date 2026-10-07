package main

import (
	"os"
	"testing"

	"github.com/olivierh59500/battlesquadron/internal/original"
	"github.com/olivierh59500/battlesquadron/internal/replay"
)

func TestSelectedRecordingCompletesFreshOrdinaryEngine(t *testing.T) {
	data, err := original.Load(os.DirFS("../../assets"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("../../assets/expert.bsinput")
	if err != nil {
		t.Fatal(err)
	}
	recording, err := replay.DecodeExpert(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if err := verify(data, recording); err != nil {
		t.Fatal(err)
	}
	// The same field count cannot establish progression when commands change.
	noActions := replay.Recording{Source: recording.Source, Fields: recording.Fields, Runs: []replay.Run{{Fields: uint32(recording.Fields)}}}
	if err := verify(data, noActions); err == nil {
		t.Fatal("missing campaign inputs accepted as the expert demonstration")
	}
	alteredDefaults := *data
	alteredDefaults.Options.Lives++
	if err := verify(&alteredDefaults, recording); err == nil {
		t.Fatal("altered starting resources accepted as an ordinary expert demonstration")
	}
}
