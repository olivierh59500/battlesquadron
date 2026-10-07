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

type pickupReference struct {
	X, Y, Subtype, Ticks int
	Hash                 string
}

// generatePickups runs the actual kind-five handler before its graphics path.
func generatePickups(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	references := []pickupReference{}
	for _, x := range []int{50, 135, 136, 210} {
		for _, subtype := range []int{0, 2, 4, 6, 10} {
			b := &bus{memory: make([]byte, 1<<24)}
			copy(b.memory[0x100:], loader)
			b.Write16(0x9b84, 304)
			b.Write32(0x2dc80, uint32((x+304)<<16))
			b.Write32(0x2dc84, 276<<16)
			b.Write32(0x2dc8c, uint32(x<<16))
			b.Write32(0x2dc88, 2<<16)
			if x >= 136 {
				b.Write32(0x2dc88, uint32(0xfffe0000))
				b.Write8(0x2dc9b, 255)
			}
			b.Write8(0x2dc9c, byte(subtype))
			b.Write8(0x2dc9f, 5)
			cpu := m68k.New(b)
			trajectory := []byte{}
			ticks := 0
			for tick := 0; tick < 512; tick++ {
				b.Write16(0x1058, uint16(2*(tick+1)))
				r := m68k.Registers{PC: 0x89ea, SR: 0x2700, SSP: 0x70000}
				r.A[7] = 0x70000
				r.A[4] = 0x2dc80
				r.A[5] = 0x8000
				cpu.SetState(r)
				for step := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x9842; step++ {
					if step > 10000 || cpu.Step() == 0 {
						panic(fmt.Sprintf("original capsule did not return at$%x", cpu.Registers().PC))
					}
				}
				if cpu.Registers().PC == 0x9842 {
					break
				}
				state := make([]byte, 15)
				binary.BigEndian.PutUint32(state, b.Read32(0x2dc8c))
				binary.BigEndian.PutUint32(state[4:], b.Read32(0x2dc84)-256<<16)
				binary.BigEndian.PutUint32(state[8:], b.Read32(0x2dc88))
				state[12] = b.Read8(0x2dc9b)
				state[13] = b.Read8(0x2dc9c)
				state[14] = b.Read8(0x2dcbf)
				trajectory = append(trajectory, state...)
				ticks++
			}
			references = append(references, pickupReference{x, 20, subtype, ticks, fmt.Sprintf("%x", sha256.Sum256(trajectory))})
		}
	}
	data, e := json.MarshalIndent(references, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(output, append(data, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Printf("Generated %d original capsule trajectories.\n", len(references))
}
