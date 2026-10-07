// Package original loads immutable native game tables recreated from the ADF.
package original

import (
	"encoding/json"
	"fmt"
	"io/fs"

	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/source"
)

// Load shares one validated table-loading path between the game and its tools.
// It never enables verification guards or changes original starting resources.
func Load(files fs.FS) (*engine.Data, error) {
	read := func(name string) ([]byte, error) { return fs.ReadFile(files, name) }
	encoded, err := read("manifest.json")
	if err != nil {
		return nil, err
	}
	var bundle source.Bundle
	if err := json.Unmarshal(encoded, &bundle); err != nil {
		return nil, err
	}
	loader, err := read(bundle.Loader)
	if err != nil {
		return nil, err
	}
	dat, err := read("unpacked/loddat.bin")
	if err != nil {
		return nil, err
	}
	if len(dat) < 0x7500 {
		return nil, fmt.Errorf("original random bank is missing")
	}
	data := &engine.Data{Loader: loader, LoaderBase: 0x100, Random: dat[0x7400:0x7500], Options: engine.DefaultOptions()}
	data.Nova, err = engine.DecodeNova(loader, 0x100, dat)
	if err != nil {
		return nil, err
	}
	overlays := make(map[string][]byte)
	for _, name := range []string{"lods0f", "lods0s", "lods0t", "lodst1", "lodst2", "lodst3"} {
		overlays[name], err = read("unpacked/" + name + ".bin")
		if err != nil {
			return nil, err
		}
	}
	schedules, err := engine.NativeSchedules(loader, 0x100, overlays)
	if err != nil {
		return nil, err
	}
	if len(bundle.Stages) != 4 {
		return nil, fmt.Errorf("original four map banks are required")
	}
	for index, spec := range bundle.Stages {
		if spec.ID != index {
			return nil, fmt.Errorf("original stage table is out of order")
		}
		name := []string{"lods0t", "lodst1", "lodst2", "lodst3"}[index]
		offset := 0x4a000 - []int{0x44000, 0x2e89a, 0x2e4c0, 0x2e840}[index]
		bank := overlays[name]
		if offset < 0 || offset+81920 > len(bank) {
			return nil, fmt.Errorf("original terrain bank%d is truncated", index)
		}
		data.Stages = append(data.Stages, engine.Stage{ID: index, Mode: spec.Mode, Height: spec.Height, Width: 24, Tiles: spec.Tiles, TileBank: bank[offset : offset+81920], Events: schedules[index], Next: (index + 1) % 4})
	}
	return data, nil
}
