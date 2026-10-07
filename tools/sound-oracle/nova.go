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

type novaReference struct {
	Owner, X, Y, ClockStart, Tick, Counter, Rays int
	ShotBank                                     bool
	Hash                                         string
}

// generateNova compares the original per-PAL ray emitter, counter and twelve
// primary-template bank writes for every update of a complete Nova activation.
func generateNova(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	resident, e := os.ReadFile(filepath.Join(assets, "loddat.bin"))
	if e != nil {
		panic(e)
	}
	game, e := os.ReadFile(filepath.Join(assets, "lodgam.bin"))
	if e != nil {
		panic(e)
	}
	references := []novaReference{}
	for owner := 0; owner < 2; owner++ {
		for _, point := range [][2]int{{40, 20}, {112, 176}, {240, 190}} {
			for _, clockStart := range []int{0, 3} {
				b := &bus{memory: make([]byte, 1<<24)}
				copy(b.memory[0x100:], loader)
				copy(b.memory[0x10000:], resident)
				copy(b.memory[0x246f0:], game)
				player := uint32(0x4da2 + owner*266)
				b.Write16(player+4, uint16(point[0]+304))
				b.Write16(player+6, uint16(point[1]+256))
				b.Write8(player+90, 255)
				b.Write8(0x1cae, 255)
				b.watchStart = player + 122
				b.watchEnd = b.watchStart + 144
				cpu := m68k.New(b)
				for tick := 1; tick <= 101; tick++ {
					b.Write16(0x1058, uint16(clockStart+tick))
					b.Write32(0x70000, 0x70010)
					b.writes = 0
					r := m68k.Registers{PC: 0x1cb0, SR: 0x2700, SSP: 0x70000}
					r.A[7] = 0x70000
					r.A[5] = 0x8000
					cpu.SetState(r)
					for step := 0; cpu.Registers().PC != 0x70010; step++ {
						if step > 1000000 || cpu.Step() == 0 {
							panic(fmt.Sprintf("Nova emitter did not return at$%x", cpu.Registers().PC))
						}
					}
					rays := 0
					state := []byte{b.Read8(0x1cae), b.Read8(player + 90)}
					for ray := uint32(0); ray < 8; ray++ {
						at := 0x48dc + ray*20
						if b.Read16(at) == 0 {
							break
						}
						rays++
						row := make([]byte, 12)
						x, y := int(b.Read16(at))-304, int(b.Read16(at+4))-256
						binary.BigEndian.PutUint16(row, uint16(x))
						binary.BigEndian.PutUint16(row[2:], uint16(y))
						binary.BigEndian.PutUint16(row[4:], uint16(x+16))
						binary.BigEndian.PutUint16(row[6:], uint16(y+int(b.Read8(at+16))))
						row[8] = b.Read8(at + 16)
						row[9] = b.Read8(at + 17)
						state = append(state, row...)
					}
					if b.writes != 0 {
						state = append(state, b.memory[b.watchStart:b.watchEnd]...)
					}
					references = append(references, novaReference{owner, point[0], point[1], clockStart, tick, int(b.Read8(0x1cae)), rays, b.writes != 0, fmt.Sprintf("%x", sha256.Sum256(state))})
				}
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
	fmt.Printf("Generated %d original Nova emitter checkpoints.\n", len(references))
}
