// Command verify exercises real extracted stages without a graphics environment.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"

	"github.com/olivierh59500/battlesquadron/assets"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/source"
)

func main() {
	frames := flag.Int("frames", 20000, "maximum PAL fields per stage")
	final := flag.Bool("final", false, "exercise the original multipart final encounter")
	flag.Parse()
	if err := run(*frames, *final); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(frames int, final bool) error {
	read := func(name string) ([]byte, error) { return fs.ReadFile(assets.Files, name) }
	encoded, err := read("manifest.json")
	if err != nil {
		return err
	}
	var bundle source.Bundle
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		return err
	}
	loader, err := read(bundle.Loader)
	if err != nil {
		return err
	}
	dat, err := read("unpacked/loddat.bin")
	if err != nil {
		return err
	}
	data := &engine.Data{Loader: loader, LoaderBase: 0x100, Random: dat[0x7400:0x7500], Options: engine.DefaultOptions()}
	data.Options.Invulnerable = true
	data.Nova, err = engine.DecodeNova(loader, 0x100, dat)
	if err != nil {
		return err
	}
	overlays := map[string][]byte{}
	for _, name := range []string{"lods0f", "lods0s", "lods0t", "lodst1", "lodst2", "lodst3"} {
		overlays[name], err = read("unpacked/" + name + ".bin")
		if err != nil {
			return err
		}
	}
	schedules, err := engine.NativeSchedules(loader, 0x100, overlays)
	if err != nil {
		return err
	}
	for _, s := range bundle.Stages {
		name := []string{"lods0t", "lodst1", "lodst2", "lodst3"}[s.ID]
		offset := 0x4a000 - []int{0x44000, 0x2e89a, 0x2e4c0, 0x2e840}[s.ID]
		data.Stages = append(data.Stages, engine.Stage{ID: s.ID, Mode: s.Mode, Height: s.Height, Width: 24, Tiles: s.Tiles, TileBank: overlays[name][offset : offset+81920], Events: schedules[s.ID]})
	}
	starts := []int{0, 1, 2, 3}
	if final {
		starts = []int{0}
	}
	for _, stage := range starts {
		core, err := engine.New(data)
		if err != nil {
			return err
		}
		core.Start(1)
		core.Players[0].Respawn = 0
		core.Players[0].Y = 170
		core.Players[0].Weapon = 3
		core.Players[0].Level = 5
		if final {
			if err := core.StartFinalBattle(); err != nil {
				return err
			}
		} else if stage != 0 {
			core.Campaign.ActiveCave = stage
			core.SelectStage(stage, 0)
		}
		maximumEnemies, maximumShots, updates := 0, 0, 0
		for updates < frames && core.Mode == engine.Playing {
			target := 128
			for _, enemy := range core.Enemies {
				if enemy.Health >= 0 && enemy.Y >= 0 && enemy.Y < 160 {
					target = enemy.X + enemy.Definition.Width/2 - 12
					break
				}
			}
			input := engine.Input{Fire: true}
			if core.Players[0].X < target-1 {
				input.X = 1
			} else if core.Players[0].X > target+1 {
				input.X = -1
			}
			core.Tick([2]engine.Input{input})
			maximumEnemies = max(maximumEnemies, len(core.Enemies))
			maximumShots = max(maximumShots, len(core.PlayerShots)+len(core.EnemyShots))
			updates++
			if !final && core.Stage != stage {
				break
			}
		}
		fmt.Printf("stage=%d final=%t fields=%d progress=%d mode=%d score=%d maximum_actors=%d maximum_projectiles=%d\n", stage, final, updates, core.Scroll, core.Mode, core.Players[0].Score, maximumEnemies, maximumShots)
		if maximumEnemies > 30 || len(core.EnemyShots) > 12 || len(core.PlayerShots) > 24 {
			return fmt.Errorf("original fixed pool bounds were exceeded")
		}
	}
	return nil
}
