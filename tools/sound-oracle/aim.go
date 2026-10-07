package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	m68k "github.com/user-none/go-chip-m68k"
)

type aimReference struct{ X, Y, Speed, VX, VY int }

// generateAim records the original MULU/DIVU/LSL shot arithmetic directly.
func generateAim(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	b := &bus{memory: make([]byte, 1<<24)}
	copy(b.memory[0x100:], loader)
	cpu := m68k.New(b)
	references := []aimReference{}
	for speed := 1; speed <= 5; speed++ {
		for _, point := range [][2]int{{0, 0}, {0, 100}, {100, 0}, {40, 100}, {100, 40}, {31, 17}, {-100, 40}, {20, -83}, {-40, -100}, {77, 77}, {255, 17}, {14, 256}} {
			dx, dy, sign := point[0], point[1], uint32(0)
			if dx < 0 {
				dx = -dx
				sign |= 1
			}
			if dy < 0 {
				dy = -dy
				sign |= 2
			}
			b.Write16(0x4734, uint16(speed<<8))
			r := m68k.Registers{PC: 0x47fc, SR: 0x2700, SSP: 0x70000}
			r.A[7] = 0x70000
			r.A[5] = 0x8000
			r.A[3] = 0x200000
			r.D[0] = uint32(speed << 16)
			r.D[1] = uint32(dx)
			r.D[2] = uint32(dy)
			r.D[7] = sign
			cpu.SetState(r)
			for step := 0; cpu.Registers().PC != 0x4854; step++ {
				if step > 10000 || cpu.Step() == 0 {
					panic("original aimed fire did not return")
				}
			}
			references = append(references, aimReference{point[0], point[1], speed, int(int32(b.Read32(0x200008))), int(int32(b.Read32(0x20000c)))})
		}
	}
	data, e := json.MarshalIndent(references, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(output, append(data, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Printf("Generated %d original aimed-shot cases.\n", len(references))
}
