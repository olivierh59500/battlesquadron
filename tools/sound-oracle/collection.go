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

type collectionReference struct {
	Owner, Weapon, Level, Nova, Subtype, Score, Cooldown, Repeat int
	Hash                                                         string
}

// generateCollection executes actual $3600 inventory collection, including the
// capped-pickup decimal adder and original weapon-header reload. Only external
// music calls are skipped; the original ship-bank and timer writes are retained.
func generateCollection(assets, output string) {
	loader, err := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if err != nil {
		panic(err)
	}
	var refs []collectionReference
	for owner := 0; owner < 2; owner++ {
		for _, weapon := range []int{0, 3} {
			for _, level := range []int{0, 4, 5} {
				for _, nova := range []int{7, 8} {
					for _, subtype := range []int{0, 2, 4, 6, 10} {
						score := []int{0, 99000, 99999000}[len(refs)%3]
						b := &bus{memory: make([]byte, 1<<24)}
						copy(b.memory[0x100:], loader)
						for index := 0; index < 2; index++ {
							p := uint32(0x4da2 + index*266)
							initialWeapon, initialLevel, initialNova, initialScore, cooldown, repeat := 2, 2, 5, 12340, 3, 4
							if index == owner {
								initialWeapon, initialLevel, initialNova, initialScore, cooldown, repeat = weapon, level, nova, score, 7, 9
							}
							b.Write16(p+58, uint16(initialWeapon))
							b.Write16(p+60, uint16(initialLevel))
							b.Write16(p+66, uint16(initialNova))
							b.Write16(p+46, uint16(cooldown))
							b.Write8(p+57, byte(repeat))
							copy(b.memory[p+106:p+114], fmt.Sprintf("%08d", initialScore))
							clear(b.memory[p+122 : p+266])
							b.Write16(p+122, 304)
							b.Write16(p+134, 304)
						}
						b.Write32(0x36a6, uint32(0x4da2+owner*266))
						b.Write16(0x2dc80, 304)
						b.Write8(0x2dc80+28, byte(subtype))
						b.Write32(0x70000, 0x70010)
						cpu := m68k.New(b)
						r := m68k.Registers{PC: 0x3600, SR: 0x2700, SSP: 0x70000}
						r.A[0], r.A[5], r.A[7] = 0x2dc80, 0x8000, 0x70000
						cpu.SetState(r)
						for steps := 0; cpu.Registers().PC != 0x70010; steps++ {
							if steps > 10000 {
								panic(fmt.Sprintf("collection routine stuck at$%x", cpu.Registers().PC))
							}
							if cpu.Registers().PC == 0x24738 || cpu.Registers().PC == 0x2473e {
								r := cpu.Registers()
								r.PC = b.Read32(r.A[7])
								r.A[7] += 4
								r.SSP = r.A[7]
								cpu.SetState(r)
								continue
							}
							if cpu.Step() == 0 {
								panic(fmt.Sprintf("collection routine stopped at$%x", cpu.Registers().PC))
							}
						}
						var state []byte
						for index := 0; index < 2; index++ {
							p := uint32(0x4da2 + index*266)
							score := 0
							for _, digit := range b.memory[p+106 : p+114] {
								score = score*10 + int(digit-'0')
							}
							shots := 0
							for slot := 0; slot < 12; slot++ {
								if b.Read16(p+122+uint32(slot*12)) != 0 {
									shots++
								}
							}
							for _, value := range []int{int(b.Read16(p + 58)), int(b.Read16(p + 60)), int(b.Read16(p + 66)), score, int(b.Read16(p + 46)), int(b.Read8(p + 57)), shots} {
								state = binary.BigEndian.AppendUint32(state, uint32(value))
							}
						}
						refs = append(refs, collectionReference{owner, weapon, level, nova, subtype, score, 7, 9, fmt.Sprintf("%x", sha256.Sum256(state))})
					}
				}
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
	fmt.Printf("Generated %d original capsule-collection fingerprints.\n", len(refs))
}
