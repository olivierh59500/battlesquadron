// Command demo recreates the ignored expert attract recording using native Go.
// It deliberately does not import assets, so it runs after a clean extraction
// even when the recording required by the application embed does not exist yet.
package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/olivierh59500/battlesquadron/internal/autoplay"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/original"
	"github.com/olivierh59500/battlesquadron/internal/replay"
)

type configuration struct {
	assets, input, output string
	fields                int
}

func main() {
	var c configuration
	flag.StringVar(&c.assets, "assets", "assets", "directory recreated by cmd/extract")
	flag.StringVar(&c.input, "input", "", "optional existing recording to verify instead of recomputing forecasts")
	flag.StringVar(&c.output, "out", "assets/expert.bsinput", "ignored generated input resource")
	flag.IntVar(&c.fields, "frames", 200000, "maximum ordinary PAL fields for deterministic planning")
	flag.Parse()
	if err := run(c); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(c configuration) error {
	if c.fields < 1 {
		return fmt.Errorf("frames must be positive")
	}
	data, err := original.Load(os.DirFS(c.assets))
	if err != nil {
		return fmt.Errorf("load original data (run go run ./cmd/extract first): %w", err)
	}
	if err := replay.CheckExpertData(data); err != nil {
		return err
	}
	var encoded []byte
	if c.input != "" {
		encoded, err = os.ReadFile(c.input)
	} else {
		encoded, err = generate(data, c.fields)
	}
	if err != nil {
		return err
	}
	recording, err := replay.DecodeExpert(encoded)
	if err != nil {
		return err
	}
	if err := verify(data, recording); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(c.output), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(c.output, encoded, 0644); err != nil {
		return err
	}
	proof := replay.ExpertMetadata()
	fmt.Printf("verified expert demo: fields=%d deaths=%d score=%d sha256=%s output=%s\n", recording.Fields, proof.Deaths, proof.Score, proof.RecordingSHA256, c.output)
	return nil
}

func generate(data *engine.Data, maximum int) ([]byte, error) {
	core, err := engine.New(data)
	if err != nil {
		return nil, err
	}
	proof := replay.ExpertMetadata()
	core.Start(proof.Players)
	pilot := autoplay.New()
	var recording replay.Recording
	source, err := hex.DecodeString(proof.SourceSHA256)
	if err != nil || len(source) != len(recording.Source) {
		return nil, fmt.Errorf("invalid expert source metadata")
	}
	copy(recording.Source[:], source)
	started := time.Now()
	for field := 0; field < maximum; field++ {
		inputs := pilot.Next(core)
		recording.Append(inputs)
		core.Tick(inputs)
		if field%2000 == 1999 {
			fmt.Printf("planning field=%d stage=%d progress=%d cleared=%02x elapsed=%s\n", core.Frame, core.Stage, core.Scroll, core.Campaign.ClearedMask, time.Since(started).Round(time.Second))
		}
		if core.Mode == engine.GameOver {
			return nil, fmt.Errorf("expert generation exhausted its ordinary ships at field %d", core.Frame)
		}
		if core.Mode == engine.Ending {
			for range 100 {
				recording.Append([2]engine.Input{})
				core.Tick([2]engine.Input{})
			}
			var encoded bytes.Buffer
			if err := replay.Encode(&encoded, recording); err != nil {
				return nil, err
			}
			return encoded.Bytes(), nil
		}
	}
	return nil, fmt.Errorf("expert generation did not complete within %d fields", maximum)
}

// verify replays every input through a fresh ordinary engine before embedding.
// It proves native progression for the selected trace, not full Amiga parity.
func verify(data *engine.Data, recording replay.Recording) error {
	if err := replay.CheckExpertData(data); err != nil {
		return err
	}
	core, err := engine.New(data)
	if err != nil {
		return err
	}
	proof := replay.ExpertMetadata()
	core.Start(proof.Players)
	cursor, err := replay.NewCursor(recording)
	if err != nil {
		return err
	}
	deaths := 0
	for {
		inputs, ok := cursor.Next()
		if !ok {
			break
		}
		core.Tick(inputs)
		for _, event := range core.Events {
			if event.Kind == "death" {
				deaths++
			}
		}
	}
	terminal := core.Digest()
	if core.Mode != engine.Ending || !core.Campaign.Completed() || core.Campaign.ClearedMask != proof.ClearedMask {
		return fmt.Errorf("expert replay did not complete all three caves and the final encounter")
	}
	if deaths != proof.Deaths || core.Players[0].Score != proof.Score || hex.EncodeToString(terminal[:]) != proof.TerminalSHA256 {
		return fmt.Errorf("expert replay diverged from the verified terminal state after %d inputs", cursor.Position())
	}
	return nil
}
