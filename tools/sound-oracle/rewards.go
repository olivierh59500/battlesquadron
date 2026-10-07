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

type rewardReference struct {
	X, Y, ClockStart, Others, Ticks int
	Hash                            string
}

func generateRewards(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	references := []rewardReference{}
	for _, x := range []int{50, 140, 200} {
		for phase := 0; phase < 4; phase++ {
			for others := 0; others < 2; others++ {
				b := &bus{memory: make([]byte, 1<<24)}
				copy(b.memory[0x100:], loader)
				b.Write16(0x9b84, 304)
				b.Write8(0x7861, 255)
				b.Write32(0x2dc80, uint32((x+304)<<16))
				b.Write32(0x2dc84, 276<<16)
				b.Write8(0x2dc9d, 8)
				b.Write32(0x2dca4, 0x11090)
				b.Write32(0x2dca0, 0x11310)
				b.Write8(0x74b4, 255)
				if others != 0 {
					b.Write16(0x2dcd0, 600)
				}
				cpu := m68k.New(b)
				trajectory := []byte{}
				ticks := 0
				for tick := 0; tick < 40; tick++ {
					b.Write16(0x1058, uint16(phase+tick+1))
					r := m68k.Registers{PC: 0x79a2, SR: 0x2700, SSP: 0x70000}
					r.A[7] = 0x70000
					r.A[4] = 0x2dc80
					r.A[5] = 0x8000
					cpu.SetState(r)
					for step := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x9774 && cpu.Registers().PC != 0x9842; step++ {
						if step > 10000 || cpu.Step() == 0 {
							panic(fmt.Sprintf("original formation reward did not return at$%x", cpu.Registers().PC))
						}
					}
					alive := cpu.Registers().PC != 0x9842
					if !alive {
						b.Write16(0x2dc80, 0)
					}
					row := make([]byte, 23)
					if alive {
						row[0] = 1
					}
					row[1] = b.Read8(0x2dc9f)
					row[2] = b.Read8(0x2dc9d)
					if alive {
						binary.BigEndian.PutUint16(row[3:], b.Read16(0x2dc80)-304)
						binary.BigEndian.PutUint16(row[5:], b.Read16(0x2dc84)-256)
					}
					if row[1] == 5 {
						binary.BigEndian.PutUint32(row[7:], b.Read32(0x2dc8c))
						binary.BigEndian.PutUint32(row[11:], b.Read32(0x2dc84)-256<<16)
						binary.BigEndian.PutUint32(row[15:], b.Read32(0x2dc88))
						row[19] = b.Read8(0x2dc9b)
						row[20] = b.Read8(0x2dc9c)
						row[21] = b.Read8(0x2dcbf)
					} else if alive {
						row[22] = byte((b.Read32(0x2dca4) - 0x11090) / 0x300)
					}
					trajectory = append(trajectory, row...)
					ticks++
					if !alive || row[1] == 5 {
						break
					}
				}
				references = append(references, rewardReference{x, 20, phase, others, ticks, fmt.Sprintf("%x", sha256.Sum256(trajectory))})
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
	fmt.Printf("Generated %d original delayed formation reward cases.\n", len(references))
}
