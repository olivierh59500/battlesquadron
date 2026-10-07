package main

import (
	"strings"
	"testing"

	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/replay"
)

func replayFixture(t *testing.T) *engine.Engine {
	t.Helper()
	data := &engine.Data{Options: engine.DefaultOptions()}
	for family := range data.Weapons {
		for level := range data.Weapons[family] {
			data.Weapons[family][level] = engine.Weapon{Shots: []engine.Shot{{VY: -6}}}
		}
	}
	core, err := engine.New(data)
	if err != nil {
		t.Fatal(err)
	}
	core.Start(1)
	return core
}

func fixtureTrace(t *testing.T) (replay.Recording, []checkpoint, string) {
	t.Helper()
	core := replayFixture(t)
	var recording replay.Recording
	var checkpoints []checkpoint
	for field := 0; field < 180; field++ {
		input := [2]engine.Input{{X: field/60 - 1, Fire: field%8 < 2}}
		recording.Append(input)
		core.Tick(input)
		if field == 89 || field == 119 {
			checkpoints = append(checkpoints, snapshot(core))
		}
	}
	return recording, checkpoints, snapshot(core).Hash
}

func TestReplayChecksIntermediateStateEvenWhenTerminalMatches(t *testing.T) {
	recording, checkpoints, terminal := fixtureTrace(t)
	if err := replayAgainst(replayFixture(t), recording, checkpoints, terminal); err != nil {
		t.Fatal(err)
	}
	checkpoints[0].Hash = strings.Repeat("0", 64)
	if err := replayAgainst(replayFixture(t), recording, checkpoints, terminal); err == nil {
		t.Fatal("corrupt intermediate state accepted despite matching terminal state")
	}
}

func TestReplayRejectsUnreachedCheckpoint(t *testing.T) {
	recording, _, terminal := fixtureTrace(t)
	checkpoints := []checkpoint{{Frame: 181}}
	if err := replayAgainst(replayFixture(t), recording, checkpoints, terminal); err == nil {
		t.Fatal("checkpoint beyond the recording was silently skipped")
	}
}

func TestReplayRejectsTerminalDivergence(t *testing.T) {
	recording, checkpoints, _ := fixtureTrace(t)
	if err := replayAgainst(replayFixture(t), recording, checkpoints, strings.Repeat("0", 64)); err == nil {
		t.Fatal("different terminal state accepted")
	}
}
