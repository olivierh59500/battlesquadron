// Command reference diagnoses bounded original FS-UAE held-input checkpoints.
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/olivierh59500/battlesquadron/assets"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/original"
	"github.com/olivierh59500/battlesquadron/internal/reference"
	"github.com/olivierh59500/battlesquadron/internal/source"
)

type fieldComparison struct {
	Name             string
	Original, Native int
	Match            bool
}

func main() {
	record := flag.String("record", "", "original FS-UAE recording")
	before := flag.String("before", "", "original initial USS checkpoint")
	after := flag.String("after", "", "original terminal USS checkpoint")
	output := flag.String("out", ".cache/validation/reference.json", "ignored diagnostic JSON report")
	right := flag.Int("right-event", 61, "observed FS-UAE joystick-right event ID")
	flag.Parse()
	if *right < 0 || *right > 65535 {
		fmt.Fprintln(os.Stderr, "right-event must fit the original 16-bit input ID")
		os.Exit(1)
	}
	if err := run(*record, *before, *after, *output, uint16(*right)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(recordPath, beforePath, afterPath, output string, right uint16) error {
	readRecord := func(path string) (reference.Recording, string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return reference.Recording{}, "", err
		}
		record, err := reference.DecodeRecording(data)
		return record, fmt.Sprintf("%x", sha256.Sum256(data)), err
	}
	record, recordHash, err := readRecord(recordPath)
	if err != nil {
		return err
	}
	readState := func(path string) (reference.State, string, int, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return reference.State{}, "", 0, err
		}
		state, err := reference.DecodeState(data)
		if err != nil {
			return state, "", 0, err
		}
		sidecar, _, err := readRecord(strings.TrimSuffix(path, ".uss") + ".fs-uae-recording")
		return state, fmt.Sprintf("%x", sha256.Sum256(data)), sidecar.LastFrame(), err
	}
	before, beforeHash, beforeFrame, err := readState(beforePath)
	if err != nil {
		return err
	}
	after, afterHash, afterFrame, err := readState(afterPath)
	if err != nil {
		return err
	}
	events, err := record.HeldRightWindow(beforeFrame, afterFrame, right)
	if err != nil {
		return err
	}
	if before.Stage != 0 || after.Stage != 0 || before.Progress >= 330 || after.Progress >= 330 {
		return fmt.Errorf("bounded comparison requires the surface opening before the first flying wave")
	}
	ticks := int(uint16(after.Clock - before.Clock))
	if ticks == 0 || ticks > 512 || afterFrame-beforeFrame <= 0 || afterFrame-beforeFrame > 512 {
		return fmt.Errorf("bounded comparison requires 1-512 logical ticks and physical recording fields")
	}
	for _, p := range before.Players {
		if !p.Active || p.Respawn != 0 || p.Dying != 0 || p.Invulnerable <= ticks {
			return fmt.Errorf("bounded comparison requires two live ships with protection throughout the window")
		}
	}
	data, err := original.Load(assets.Files)
	if err != nil {
		return err
	}
	core, err := engine.New(data)
	if err != nil {
		return err
	}
	core.Start(2)
	core.Frame, core.Stage, core.Scroll, core.CameraX, core.Players = before.Clock, before.Stage, before.Progress, before.Camera, before.Players
	core.Campaign.ClearedMask = before.Cleared
	for tick := 0; tick < ticks; tick++ {
		core.Tick([2]engine.Input{{X: 1}, {}})
	}
	var fields []fieldComparison
	compare := func(name string, original, native int) {
		fields = append(fields, fieldComparison{name, original, native, original == native})
	}
	compare("clock", after.Clock, core.Frame&65535)
	compare("terrain", after.Progress, core.Scroll)
	compare("camera", after.Camera, core.CameraX)
	for index, p := range after.Players {
		n := core.Players[index]
		for _, value := range []struct {
			name             string
			original, native int
		}{{"x", p.X, n.X}, {"y", p.Y, n.Y}, {"tilt", p.Tilt, n.Tilt}, {"lives", p.Lives, n.Lives}, {"weapon", p.Weapon, n.Weapon}, {"level", p.Level, n.Level}, {"nova", p.Nova, n.Nova}, {"score", p.Score, n.Score}, {"invulnerability", p.Invulnerable, n.Invulnerable}, {"respawn", p.Respawn, n.Respawn}, {"dying", p.Dying, n.Dying}, {"cooldown", p.Cooldown, n.Cooldown}, {"repeat", p.Repeat, n.Repeat}} {
			compare(fmt.Sprintf("player%d.%s", index+1, value.name), value.original, value.native)
		}
	}
	if before.CycleUnit != after.CycleUnit || before.CycleUnit != 512 || after.StoredCycles <= before.StoredCycles {
		return fmt.Errorf("cycle provenance is inconsistent")
	}
	delta := after.StoredCycles - before.StoredCycles
	cyclesPerField := uint64(before.CycleUnit) * 227 * 313
	if delta%cyclesPerField != 0 || delta/cyclesPerField != uint64(afterFrame-beforeFrame) {
		return fmt.Errorf("physical PAL cycle delta does not match the recorded field count")
	}
	phase := "Source phase correspondence has not been established."
	if before.PC == 0xce0 && after.PC == 0xbae {
		phase = "Initial PC$0CE0 follows both player passes; terminal PC$0BAE follows the next terrain/camera/NPC pass and precedes its first player pass. One extra source terrain/camera step is present between these mixed-phase saves."
	}
	if before.PC == after.PC {
		phase = "Both checkpoints have the same original instruction PC; broader raster and controller correspondence still requires independent verification."
	}
	postPlayerWait := func(pc uint32) bool {
		return pc == 0xcce || pc == 0xcd2 || pc == 0xcd8 || pc == 0xcde || pc == 0xce0 || pc == 0xce6
	}
	if postPlayerWait(before.PC) && postPlayerWait(after.PC) {
		phase = "Both checkpoints are in the original post-player raster-wait loop$0CCE..$0CE6. No gameplay-state writes occur between these instruction positions; the selected scalars have a corresponding complete-loop phase."
	}
	proof := struct {
		Scope, Source, RecordHash, BeforeHash, AfterHash, Phase string
		Before, After                                           reference.State
		BeforeFrame, AfterFrame, LogicalTicks                   int
		CycleDelta, StoredCyclesPerPALField                     uint64
		PhysicalPALFields                                       float64
		Events                                                  []reference.Event
		Comparisons                                             []fieldComparison
	}{Scope: "Bounded held-right diagnostic seeded from original ship/camera scalars. Enemy/projectile pools and composite pixels are not reconstructed; this does not establish whole-game matched-input parity.", Source: source.DiskSHA256, RecordHash: recordHash, BeforeHash: beforeHash, AfterHash: afterHash, Phase: phase, Before: before, After: after, BeforeFrame: beforeFrame, AfterFrame: afterFrame, LogicalTicks: ticks, CycleDelta: delta, StoredCyclesPerPALField: cyclesPerField, PhysicalPALFields: float64(delta) / float64(cyclesPerField), Events: events, Comparisons: fields}
	encoded, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(output, append(encoded, '\n'), 0644); err != nil {
		return err
	}
	matches := 0
	for _, field := range fields {
		if field.Match {
			matches++
		}
	}
	fmt.Printf("Compared %d logical ticks across %.0f physical PAL fields: %d/%d scalar fields match. Diagnostic: %s\n", ticks, proof.PhysicalPALFields, matches, len(fields), output)
	return nil
}
