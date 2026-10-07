package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	m68k "github.com/user-none/go-chip-m68k"
)

type lifeReference struct {
	Previous, Current, Spare, Clock int
	Hash                            string
}

// generateLives executes the actual extra-life routine at $1340. Only its
// external sound call is skipped; ship-count and score-digit writes come from
// the original instructions, including the selected player's display update.
func generateLives(assets, output string) {
	loader, err := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if err != nil {
		panic(err)
	}
	pairs := [][2]int{{0, 0}, {99999, 100010}, {199999, 200005}, {299999, 300001}, {599999, 600020}, {90000, 310000}, {999999, 1000000}, {1999999, 2000020}, {9999999, 10000000}, {99999999, 0}}
	var refs []lifeReference
	for _, pair := range pairs {
		for _, spare := range []int{0, 3, 4, 5} {
			for _, clock := range []int{1, 5} {
				b := &bus{memory: make([]byte, 1<<24)}
				copy(b.memory[0x100:], loader)
				owner := clock >> 2 & 1
				for index := 0; index < 2; index++ {
					p := uint32(0x4da2 + index*266)
					previous, current := 42000, 42000
					if index == owner {
						previous, current = pair[0], pair[1]
					}
					b.Write8(p+56, byte(spare))
					copy(b.memory[p+106:p+114], fmt.Sprintf("%08d", current))
					copy(b.memory[p+114:p+118], fmt.Sprintf("%08d", previous)[:4])
				}
				b.Write16(0x1058, uint16(clock))
				b.Write32(0x70000, 0x70010)
				cpu := m68k.New(b)
				r := m68k.Registers{PC: 0x1340, SR: 0x2700, SSP: 0x70000}
				r.A[5], r.A[7] = 0x8000, 0x70000
				cpu.SetState(r)
				awarded := byte(0)
				for steps := 0; cpu.Registers().PC != 0x70010; steps++ {
					if steps > 10000 {
						panic(fmt.Sprintf("extra-life routine stuck at$%x", cpu.Registers().PC))
					}
					if cpu.Registers().PC == 0x24744 {
						awarded = 1
						r := cpu.Registers()
						r.PC = b.Read32(r.A[7])
						r.A[7] += 4
						r.SSP = r.A[7]
						cpu.SetState(r)
						continue
					}
					if cpu.Step() == 0 {
						panic(fmt.Sprintf("extra-life routine stopped at$%x", cpu.Registers().PC))
					}
				}
				state := []byte{awarded}
				for index := 0; index < 2; index++ {
					p := uint32(0x4da2 + index*266)
					state = append(state, b.Read8(p+56))
					state = append(state, b.memory[p+114:p+118]...)
				}
				refs = append(refs, lifeReference{pair[0], pair[1], spare, clock, fmt.Sprintf("%x", sha256.Sum256(state))})
			}
		}
	}
	data, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %d original extra-life fingerprints.\n", len(refs))
}
