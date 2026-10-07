package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	m68k "github.com/user-none/go-chip-m68k"
)

type waveReference struct {
	Stage, Event, Ticks int
	Hash                string
}

// generateWaves executes only the original directed formation update, stopping
// before its graphics path. It compares behavior, not a second script decoder.
func generateWaves(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	references := []waveReference{}
	for stage, name := range []string{"lods0f", "lodst1", "lodst2", "lodst3"} {
		overlay, e := os.ReadFile(filepath.Join(assets, name+".bin"))
		if e != nil {
			panic(e)
		}
		bankBase := []uint32{0x2e508, 0x2e89a, 0x2e4c0, 0x2e840}[stage]
		template := make([]byte, 1<<24)
		copy(template[0x100:], loader)
		copy(template[bankBase:], overlay)
		pointer := binary.BigEndian.Uint32(template[0x148e+stage*140:])
		for event := 0; event < 4096; event++ {
			row := template[pointer : pointer+12]
			if binary.BigEndian.Uint16(row) == 65535 {
				break
			}
			pointer += 12
			if row[6] != 0 || binary.BigEndian.Uint16(row[2:]) == 65535 {
				continue
			}
			script := binary.BigEndian.Uint32(row[8:])
			if stage == 1 && script >= 0xf068 && script <= 0xf0e6 && (script-0xf068)%18 == 0 {
				script = 0xe88a + (script - 0xf068)
			}
			b := &bus{memory: append([]byte(nil), template...)}
			b.Write32(0x2dc80, 288<<16)
			b.Write32(0x2dc84, 20<<16)
			b.Write32(0x2dc8c, script)
			cpu := m68k.New(b)
			trajectory := []byte{}
			ticks := 0
			for tick := 0; tick < 512; tick++ {
				r := m68k.Registers{PC: 0x96b2, SR: 0x2700, SSP: 0x70000}
				r.A[7] = 0x70000
				r.A[4] = 0x2dc80
				r.A[5] = 0x8000
				cpu.SetState(r)
				for step := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x9842; step++ {
					if step > 10000 || cpu.Step() == 0 {
						panic(fmt.Sprintf("directed handler did not return stage%d event%d at$%x", stage, event, cpu.Registers().PC))
					}
				}
				if cpu.Registers().PC == 0x9842 {
					break
				}
				trajectory = append(trajectory, b.memory[0x2dc80:0x2dc88]...)
				trajectory = append(trajectory, b.memory[0x2dcbf])
				ticks++
			}
			references = append(references, waveReference{stage, event, ticks, fmt.Sprintf("%x", sha256.Sum256(trajectory))})
		}
	}
	data, e := json.MarshalIndent(references, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(output, append(data, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Printf("Generated %d original directed-flight trajectory fingerprints.\n", len(references))
}
