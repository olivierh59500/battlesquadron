package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/original"
	"github.com/olivierh59500/battlesquadron/internal/replay"
	"github.com/olivierh59500/battlesquadron/internal/source"
	m68k "github.com/user-none/go-chip-m68k"
)

// compareRecordedLives supplies the actual sampled values from a native input
// replay to original $1340 instructions. Each routine result is independently
// compared, without treating the surrounding native game as an original game.
func compareRecordedLives(assets, input, output string, players int) error {
	if players < 1 || players > 2 {
		return fmt.Errorf("recorded life comparison requires one or two players")
	}
	encoded, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	file, err := os.Open(input)
	if err != nil {
		return err
	}
	recording, err := replay.Decode(file)
	file.Close()
	if err != nil {
		return err
	}
	expectedSource, err := hex.DecodeString(source.DiskSHA256)
	if err != nil || string(recording.Source[:]) != string(expectedSource) {
		return fmt.Errorf("input recording belongs to a different original ADF")
	}
	data, err := original.Load(os.DirFS(filepath.Dir(assets)))
	if err != nil {
		return err
	}
	core, err := engine.New(data)
	if err != nil {
		return err
	}
	core.Start(players)
	b := &bus{memory: make([]byte, 1<<24)}
	copy(b.memory[0x100:], data.Loader)
	cpu := m68k.New(b)
	actualHash, originalHash := sha256.New(), sha256.New()
	checked, skipped, awards := 0, 0, 0
	var processed uint64
	for _, run := range recording.Runs {
		for field := uint32(0); field < run.Fields; field++ {
			processed++
			previous := [2]int{}
			for index := range previous {
				previous[index], _ = core.PlayerLifeSample(index)
			}
			stage, playing := core.Stage, core.Mode == engine.Playing
			core.Tick(run.Inputs)
			if !playing || core.Frame&1 == 0 {
				continue
			}
			if core.Stage != stage {
				// Transition payouts run after the sampled routine and change the
				// score again. A post-tick snapshot cannot reconstruct that seam.
				skipped++
				continue
			}
			owner := core.Clock() >> 2 & 1
			_, resultingSpare := core.PlayerLifeSample(owner)
			nativeAward := byte(0)
			for _, event := range core.Events {
				if event.Kind == "extra-life" && event.Player == owner {
					nativeAward = 1
					awards++
				}
			}
			beforeSpare := resultingSpare - int(nativeAward)
			p := uint32(0x4da2 + owner*266)
			b.Write8(p+56, byte(beforeSpare))
			copy(b.memory[p+106:p+114], fmt.Sprintf("%08d", core.Players[owner].Score))
			copy(b.memory[p+114:p+118], fmt.Sprintf("%08d", previous[owner])[:4])
			b.Write16(0x1058, uint16(core.Clock()))
			b.Write32(0x70000, 0x70010)
			r := m68k.Registers{PC: 0x1340, SR: 0x2700, SSP: 0x70000}
			r.A[5], r.A[7] = 0x8000, 0x70000
			cpu.SetState(r)
			originalAward := byte(0)
			for steps := 0; cpu.Registers().PC != 0x70010; steps++ {
				if steps > 10000 {
					return fmt.Errorf("recorded life routine stuck at$%x", cpu.Registers().PC)
				}
				if cpu.Registers().PC == 0x24744 {
					originalAward = 1
					r := cpu.Registers()
					r.PC = b.Read32(r.A[7])
					r.A[7] += 4
					r.SSP = r.A[7]
					cpu.SetState(r)
					continue
				}
				if cpu.Step() == 0 {
					return fmt.Errorf("recorded life routine stopped at$%x", cpu.Registers().PC)
				}
			}
			want := append([]byte{originalAward, b.Read8(p + 56)}, b.memory[p+114:p+118]...)
			lastSample, _ := core.PlayerLifeSample(owner)
			got := append([]byte{nativeAward, byte(resultingSpare)}, fmt.Sprintf("%08d", lastSample)[:4]...)
			if string(got) != string(want) {
				return fmt.Errorf("extra-life seam differs at field%d stage%d progress%d: native%x original%x, previous score%d current score%d", core.Frame, core.Stage, core.Scroll, got, want, previous[owner], core.Players[owner].Score)
			}
			actualHash.Write(got)
			originalHash.Write(want)
			checked++
		}
	}
	proof := struct {
		Scope, Source, InputHash, LoaderHash, GoVersion, OriginalHash, NativeHash string
		Fields                                                                    uint64
		Players, Checked, SkippedTransitions, Awards                              int
	}{
		Scope:  "Actual native replay samples compared with original $1340 instructions; surrounding progression and score generation remain native and this is not an original full-game replay.",
		Source: source.DiskSHA256, InputHash: fmt.Sprintf("%x", sha256.Sum256(encoded)), LoaderHash: fmt.Sprintf("%x", sha256.Sum256(data.Loader)), GoVersion: runtime.Version(),
		OriginalHash: hex.EncodeToString(originalHash.Sum(nil)), NativeHash: hex.EncodeToString(actualHash.Sum(nil)), Fields: processed, Players: players, Checked: checked, SkippedTransitions: skipped, Awards: awards,
	}
	proofData, err := json.MarshalIndent(proof, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(output, append(proofData, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("Compared %d actual recorded extra-life samples and %d awards across %d input fields; transition seams skipped %d.\n", checked, awards, processed, skipped)
	return nil
}
