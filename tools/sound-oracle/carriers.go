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

type carrierReference struct {
	X, Y, ClockStart, ClockStep, Random, Ticks int
	Hash                                       string
}

// generateCarriers executes kind-six's actual custom death frames and its
// $98E6 random-weapon transformation, through the capsule's immediate update.
func generateCarriers(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	resident, e := os.ReadFile(filepath.Join(assets, "loddat.bin"))
	if e != nil {
		panic(e)
	}
	references := []carrierReference{}
	for _, x := range []int{50, 140, 200} {
		for phase := 0; phase < 4; phase++ {
			for _, clockStep := range []int{1, 2} {
				for _, random := range []int{0, 17, 233} {
					b := &bus{memory: make([]byte, 1<<24)}
					copy(b.memory[0x100:], loader)
					copy(b.memory[0x10000:], resident)
					b.Write16(0x9b84, 304)
					b.Write8(0x7861, 255)
					b.Write8(0x2aa5, byte(random))
					b.Write32(0x2dc80, uint32((x+304)<<16))
					b.Write32(0x2dc84, 276<<16)
					b.Write8(0x2dc9f, 6)
					b.Write8(0x2dcbf, 5)
					b.Write8(0x74b4, 255)
					cpu := m68k.New(b)
					trajectory := []byte{}
					ticks := 0
					for tick := 0; tick < 40; tick++ {
						b.Write16(0x1058, uint16(phase+clockStep*tick))
						r := m68k.Registers{PC: 0x886c, SR: 0x2700, SSP: 0x70000}
						r.A[7] = 0x70000
						r.A[4] = 0x2dc80
						r.A[5] = 0x8000
						cpu.SetState(r)
						for step := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x9842; step++ {
							if step > 10000 || cpu.Step() == 0 {
								panic(fmt.Sprintf("original carrier reward did not return at$%x", cpu.Registers().PC))
							}
						}
						row := make([]byte, 23)
						row[0] = 1
						row[1] = b.Read8(0x2dc9f)
						binary.BigEndian.PutUint16(row[3:], b.Read16(0x2dc80)-304)
						binary.BigEndian.PutUint16(row[5:], b.Read16(0x2dc84)-256)
						if row[1] == 5 {
							binary.BigEndian.PutUint32(row[7:], b.Read32(0x2dc8c))
							binary.BigEndian.PutUint32(row[11:], b.Read32(0x2dc84)-256<<16)
							binary.BigEndian.PutUint32(row[15:], b.Read32(0x2dc88))
							row[19] = b.Read8(0x2dc9b)
							row[20] = b.Read8(0x2dc9c)
							row[21] = b.Read8(0x2dcbf)
						} else {
							row[2] = 11 - b.Read8(0x2dcbf)
							row[22] = b.Read8(0x2dcbf)
						}
						trajectory = append(trajectory, row...)
						ticks++
						if row[1] == 5 {
							break
						}
					}
					references = append(references, carrierReference{x, 20, phase, clockStep, random, ticks, fmt.Sprintf("%x", sha256.Sum256(trajectory))})
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
	fmt.Printf("Generated %d original weapon-carrier reward cases.\n", len(references))
}
