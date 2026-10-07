// Command verify records and replays an input-only expert campaign.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/olivierh59500/battlesquadron/assets"
	"github.com/olivierh59500/battlesquadron/internal/autoplay"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/original"
	"github.com/olivierh59500/battlesquadron/internal/replay"
	"github.com/olivierh59500/battlesquadron/internal/source"
)

type configuration struct {
	mode, report, record, input string
	fields, players             int
	final                       bool
}
type checkpoint struct {
	Frame, Stage, Progress, Camera int
	Mode                           engine.Mode
	Cleared                        uint8
	Final                          bool
	Players                        [2]engine.Player
	Hash                           string
}
type report struct {
	Mode, Source, OriginalProgramSHA256, GoVersion         string
	Options                                                engine.Options
	Starting, Terminal                                     checkpoint
	Checkpoints                                            []checkpoint
	Events                                                 map[string]int
	Plans                                                  autoplay.Statistics
	MaximumActors, MaximumPlayerShots, MaximumHostileShots int
	Completed, DeterministicReplay                         bool
	ReplayFields                                           uint64
	ElapsedSeconds                                         float64
	Scope                                                  string
}

func main() {
	c := configuration{}
	flag.StringVar(&c.mode, "mode", "expert", "expert, replay, or inspect")
	flag.IntVar(&c.fields, "frames", 200000, "maximum ordinary PAL fields")
	flag.IntVar(&c.players, "players", 1, "one or two ordinary human ships")
	flag.StringVar(&c.report, "report", ".cache/validation/expert.json", "ignored JSON validation report")
	flag.StringVar(&c.record, "record", ".cache/validation/expert.bsinput", "ignored run-length input recording")
	flag.StringVar(&c.input, "input", "", "input recording for replay mode")
	flag.BoolVar(&c.final, "final", false, "inspect mode only: begin at the final encounter")
	flag.Parse()
	if err := run(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func snapshot(e *engine.Engine) checkpoint {
	digest := e.Digest()
	return checkpoint{Frame: e.Frame, Stage: e.Stage, Progress: e.Scroll, Camera: e.CameraX, Mode: e.Mode, Cleared: e.Campaign.ClearedMask, Final: e.FinalBattle(), Players: e.Players, Hash: hex.EncodeToString(digest[:])}
}

func run(c configuration) error {
	if c.players < 1 || c.players > 2 || c.fields < 1 {
		return fmt.Errorf("players must be 1-2 and frames must be positive")
	}
	if c.mode != "expert" && c.mode != "replay" && c.mode != "inspect" {
		return fmt.Errorf("unknown verifier mode %q", c.mode)
	}
	if c.final && c.mode != "inspect" {
		return fmt.Errorf("-final changes starting state and is allowed only in inspect mode")
	}
	data, err := original.Load(assets.Files)
	if err != nil {
		return err
	}
	core, err := engine.New(data)
	if err != nil {
		return err
	}
	core.Start(c.players)
	if c.mode == "inspect" {
		core.Options.Invulnerable = true
		for index := range core.Players {
			if core.Players[index].Active {
				core.Players[index].Weapon = 3
				core.Players[index].Level = 5
				core.Players[index].Respawn = 0
				core.Players[index].Y = 160
			}
		}
		if c.final {
			if err := core.StartFinalBattle(); err != nil {
				return err
			}
		}
	}
	if c.mode == "replay" {
		if c.input == "" {
			return fmt.Errorf("replay mode requires -input")
		}
		file, err := os.Open(c.input)
		if err != nil {
			return err
		}
		recording, err := replay.Decode(file)
		file.Close()
		if err != nil {
			return err
		}
		return replayRun(core, recording, c.report)
	}
	var recording replay.Recording
	sourceBytes, _ := hex.DecodeString(source.DiskSHA256)
	copy(recording.Source[:], sourceBytes)
	r := report{Mode: c.mode, Source: source.DiskSHA256, OriginalProgramSHA256: sha256String(data.Loader), GoVersion: runtime.Version(), Options: core.Options, Starting: snapshot(core), Events: make(map[string]int), Scope: "Native no-cheat progression and deterministic input replay; original routine comparisons are separate. This report does not claim complete FS-UAE campaign parity."}
	if c.mode == "inspect" {
		r.Scope = "Diagnostic progression with altered starting resources and invulnerability; this report cannot establish ordinary progression or complete Amiga parity."
	}
	pilot := autoplay.New()
	started := time.Now()
	previous := r.Starting
	for field := 0; field < c.fields; field++ {
		inputs := pilot.Next(core)
		recording.Append(inputs)
		core.Tick(inputs)
		for _, event := range core.Events {
			r.Events[event.Kind]++
		}
		r.MaximumActors = max(r.MaximumActors, len(core.Enemies)+len(core.Pickups))
		r.MaximumPlayerShots = max(r.MaximumPlayerShots, len(core.PlayerShots))
		r.MaximumHostileShots = max(r.MaximumHostileShots, len(core.EnemyShots))
		if len(core.PlayerShots) > 24 || len(core.EnemyShots) > 12 {
			return fmt.Errorf("original projectile pool bounds exceeded at field %d", core.Frame)
		}
		current := snapshotIfNeeded(core, previous, field%1000 == 999)
		if current != nil {
			r.Checkpoints = append(r.Checkpoints, *current)
			previous = *current
		}
		if field%2000 == 1999 {
			fmt.Printf("field=%d stage=%d progress=%d mask=%02x score=%d ships=%d weapon=%d level=%d capsules=%d wrecks=%d elapsed=%s\n", core.Frame, core.Stage, core.Scroll, core.Campaign.ClearedMask, core.Players[0].Score, core.Players[0].Lives, core.Players[0].Weapon, core.Players[0].Level, r.Events["pickup"], r.Events["wreck-bonus"], time.Since(started).Round(time.Millisecond))
		}
		if core.Mode == engine.GameOver {
			break
		}
		if core.Mode == engine.Ending {
			for range 100 {
				recording.Append([2]engine.Input{})
				core.Tick([2]engine.Input{})
			}
			break
		}
	}
	r.Terminal = snapshot(core)
	r.Completed = core.Mode == engine.Ending && core.Campaign.Completed()
	r.Plans = pilot.Statistics
	r.ElapsedSeconds = time.Since(started).Seconds()
	r.ReplayFields = recording.Fields
	if c.record != "" {
		if err := writeRecording(c.record, recording); err != nil {
			return err
		}
	}
	// A second, input-only pass proves that planning never supplied state edits.
	if c.mode == "expert" {
		fresh, err := engine.New(data)
		if err != nil {
			return err
		}
		fresh.Start(c.players)
		if err := replayAgainst(fresh, recording, r.Checkpoints, r.Terminal.Hash); err != nil {
			return err
		}
		r.DeterministicReplay = true
	}
	if err := writeReport(c.report, r); err != nil {
		return err
	}
	fmt.Printf("completed=%t fields=%d deaths=%d ships=%d score=%d capsules=%d buildings=%d wrecks=%d novas=%d replay=%t report=%s\n", r.Completed, core.Frame, r.Events["death"], core.Players[0].Lives, core.Players[0].Score, r.Events["pickup"], r.Events["building-destroyed"], r.Events["wreck-bonus"], r.Events["nova"], r.DeterministicReplay, c.report)
	if !r.Completed {
		return fmt.Errorf("expert campaign did not complete within %d fields", c.fields)
	}
	return nil
}

func snapshotIfNeeded(e *engine.Engine, previous checkpoint, periodic bool) *checkpoint {
	if !periodic && e.Stage == previous.Stage && e.Mode == previous.Mode && e.Campaign.ClearedMask == previous.Cleared && e.FinalBattle() == previous.Final {
		return nil
	}
	c := snapshot(e)
	return &c
}

func replayAgainst(e *engine.Engine, recording replay.Recording, checkpoints []checkpoint, terminalHash string) error {
	index := 0
	for _, run := range recording.Runs {
		for field := uint32(0); field < run.Fields; field++ {
			e.Tick(run.Inputs)
			for index < len(checkpoints) && e.Frame >= checkpoints[index].Frame {
				if e.Frame != checkpoints[index].Frame {
					return fmt.Errorf("input-only replay missed checkpoint at field %d", checkpoints[index].Frame)
				}
				actual := snapshot(e)
				if actual.Hash != checkpoints[index].Hash {
					return fmt.Errorf("input-only replay diverged at field %d", e.Frame)
				}
				index++
			}
		}
	}
	if index != len(checkpoints) {
		return fmt.Errorf("input-only replay stopped before checkpoint at field %d", checkpoints[index].Frame)
	}
	if actual := snapshot(e); actual.Hash != terminalHash {
		return fmt.Errorf("input-only replay terminal state diverged")
	}
	return nil
}

func replayRun(e *engine.Engine, recording replay.Recording, path string) error {
	var expected [32]byte
	bytes, _ := hex.DecodeString(source.DiskSHA256)
	copy(expected[:], bytes)
	if recording.Source != expected {
		return fmt.Errorf("input recording targets a different source disk")
	}
	r := report{Mode: "replay", Source: source.DiskSHA256, OriginalProgramSHA256: sha256String(e.Data.Loader), GoVersion: runtime.Version(), Options: e.Options, Starting: snapshot(e), Events: make(map[string]int), ReplayFields: recording.Fields, Scope: "Input-only replay of the native engine; original CPU parity is checked separately."}
	started := time.Now()
	for _, run := range recording.Runs {
		for field := uint32(0); field < run.Fields; field++ {
			e.Tick(run.Inputs)
			for _, event := range e.Events {
				r.Events[event.Kind]++
			}
		}
	}
	r.Terminal = snapshot(e)
	r.Completed = e.Mode == engine.Ending && e.Campaign.Completed()
	r.ElapsedSeconds = time.Since(started).Seconds()
	return writeReport(path, r)
}

func writeRecording(path string, r replay.Recording) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return replay.Encode(file, r)
}
func writeReport(path string, r report) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}
func sha256String(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
