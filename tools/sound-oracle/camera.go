package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	m68k "github.com/user-none/go-chip-m68k"
)

type cameraReference struct {
	X                     [2]int
	Marker                [2]byte
	Before, Target, After int
}

func generateCamera(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	b := &bus{memory: make([]byte, 1<<24)}
	copy(b.memory[0x100:], loader)
	b.Write16(0xa0a2, 0)
	cpu := m68k.New(b)
	references := []cameraReference{}
	for _, points := range [][2]int{{0, 0}, {112, 160}, {256, 256}, {256, 0}, {10, 244}} {
		for _, markers := range [][2]byte{{0, 0}, {0, 255}, {255, 0}, {255, 255}, {100, 150}, {150, 100}, {150, 150}, {100, 100}} {
			for _, camera := range []int{0, 48, 96} {
				for index := 0; index < 2; index++ {
					player := uint32(0x4da2 + index*266)
					b.Write16(player, uint16(points[index]+256))
					b.Write8(player+38, markers[index])
				}
				b.Write16(0x9b84, uint16(camera+256))
				r := m68k.Registers{PC: 0x9ba4, SR: 0x2700, SSP: 0x70000}
				r.A[7] = 0x70000
				r.A[5] = 0x8000
				cpu.SetState(r)
				for step := 0; cpu.Registers().PC != 0x9c0e; step++ {
					if step > 10000 || cpu.Step() == 0 {
						panic("original camera formula did not return")
					}
				}
				references = append(references, cameraReference{points, markers, camera, int(uint16(cpu.Registers().D[1])) - 256, int(b.Read16(0x9b84)) - 256})
			}
		}
	}
	data, e := json.MarshalIndent(references, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(output, append(data, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Printf("Generated %d original camera cases.\n", len(references))
}
