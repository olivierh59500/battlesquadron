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

type controllerReference struct {
	Kind, Stage, X, Y, Ticks int
	Hash                     string
	Damage                   bool
	ClockStart, ClockStep    int
	Fires                    []fireReference `json:",omitempty"`
}

type fireReference struct{ Tick, X, Y, VX, VY int }

func generateControllers(assets, output string) {
	loader, e := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if e != nil {
		panic(e)
	}
	resident, e := os.ReadFile(filepath.Join(assets, "loddat.bin"))
	if e != nil {
		panic(e)
	}
	references := []controllerReference{}
	for _, kind := range []int{1, 4, 6} {
		for stage := 0; stage < 2; stage++ {
			for _, point := range [][2]int{{40, -32}, {160, -32}, {100, 0}, {200, 120}} {
				for _, damage := range []bool{false, true} {
					for _, clockStep := range []int{1, 2} {
						clockStart := 1
						if clockStep == 2 {
							clockStart = 0
						}
						b := &bus{memory: make([]byte, 1<<24)}
						copy(b.memory[0x100:], loader)
						copy(b.memory[0x10000:], resident)
						b.Write16(0x9b84, 304)
						b.Write16(0x9b96, 1)
						b.Write16(0x9b9c, uint16(stage))
						b.Write8(0x76ca, 50)
						for i, x := range []int{112, 160} {
							player := uint32(0x4da2 + i*266)
							b.Write16(player+4, uint16(x+304))
							b.Write16(player+6, 176+256)
							b.Write8(player+38, 10)
						}
						b.Write32(0x2dc80, uint32((point[0]+304)<<16))
						b.Write32(0x2dc84, uint32((point[1]+256)<<16))
						b.Write8(0x2dc9f, byte(kind))
						b.Write8(0x2dc98, 10)
						cpu := m68k.New(b)
						trajectory := []byte{}
						ticks := 0
						fires := []fireReference{}
						for tick := 0; tick < 240; tick++ {
							if damage && tick == 40 {
								b.Write8(0x2dcbe, 1)
							}
							b.Write16(0x1058, uint16(clockStart+clockStep*tick))
							entry := uint32(0x8aa8)
							if kind == 1 {
								entry = 0x9538
							}
							if kind == 6 {
								entry = 0x886c
							}
							r := m68k.Registers{PC: entry, SR: 0x2700, SSP: 0x70000}
							r.A[7] = 0x70000
							r.A[4] = 0x2dc80
							r.A[5] = 0x8000
							cpu.SetState(r)
							for step := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x9842; step++ {
								if step > 10000 || cpu.Step() == 0 {
									panic(fmt.Sprintf("controller%d did not return at$%x", kind, cpu.Registers().PC))
								}
							}
							if cpu.Registers().PC == 0x9842 {
								break
							}
							if b.Read8(0x2dc9e)&32 != 0 {
								b.Write8(0x2dc9e, b.Read8(0x2dc9e)&^32)
								sx, sy := int(b.Read16(0x2dcba)), int(b.Read16(0x2dcbc))
								dx, dy := int(int16(b.Read16(0x4730))), int(int16(b.Read16(0x4732)))
								if dx == 0 {
									targetX := 112 + 304 + 12
									otherX := 160 + 304 + 12
									targetY := 176 + 256 + 16
									if absOracle(otherX-sx)+absOracle(targetY-sy) <= absOracle(targetX-sx)+absOracle(targetY-sy) {
										targetX = otherX
									}
									dx, dy = targetX-sx, targetY-sy
								} else {
									b.Write16(0x4730, 0)
								}
								sign := uint32(0)
								if dx < 0 {
									dx = -dx
									sign |= 1
								}
								if dy < 0 {
									dy = -dy
									sign |= 2
								}
								b.Write16(0x4734, 512)
								r := m68k.Registers{PC: 0x47fc, SR: 0x2700, SSP: 0x70000}
								r.A[7] = 0x70000
								r.A[5] = 0x8000
								r.A[3] = 0x200000
								r.D[0] = 2 << 16
								r.D[1] = uint32(dx)
								r.D[2] = uint32(dy)
								r.D[7] = sign
								cpu.SetState(r)
								for step := 0; cpu.Registers().PC != 0x4854; step++ {
									if step > 10000 || cpu.Step() == 0 {
										panic("original controller shot did not return")
									}
								}
								fires = append(fires, fireReference{tick, sx - 304, sy - 256, int(int32(b.Read32(0x200008))), int(int32(b.Read32(0x20000c)))})
							}
							state := make([]byte, 21)
							binary.BigEndian.PutUint32(state, b.Read32(0x2dc80)-304<<16)
							binary.BigEndian.PutUint32(state[4:], b.Read32(0x2dc84)-256<<16)
							binary.BigEndian.PutUint32(state[8:], b.Read32(0x2dc88))
							binary.BigEndian.PutUint32(state[12:], b.Read32(0x2dc8c))
							if kind == 1 {
								binary.BigEndian.PutUint32(state[8:], b.Read32(0x2dc8c))
								binary.BigEndian.PutUint32(state[12:], 0)
							}
							state[16] = b.Read8(0x2dcbf)
							if kind == 4 {
								state[17] = b.Read8(0x2dc9a)
							}
							state[18] = b.Read8(0x2dc9b)
							state[19] = b.Read8(0x2dc9c)
							state[20] = b.Read8(0x2aa5)
							trajectory = append(trajectory, state...)
							ticks++
						}
						references = append(references, controllerReference{Kind: kind, Stage: stage, X: point[0], Y: point[1], Ticks: ticks, Hash: fmt.Sprintf("%x", sha256.Sum256(trajectory)), Damage: damage, Fires: fires, ClockStart: clockStart, ClockStep: clockStep})
						directory := filepath.Join(assets, "../../.cache/sound-oracle/controller-dumps")
						if e = os.MkdirAll(directory, 0755); e != nil {
							panic(e)
						}
						if e = os.WriteFile(filepath.Join(directory, fmt.Sprintf("%d-%d-%d-%d-%v.bin", kind, stage, point[0], point[1], damage)), trajectory, 0644); e != nil {
							panic(e)
						}
					}
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
	fmt.Printf("Generated %d original adaptive-controller fingerprints.\n", len(references))
}

func absOracle(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
