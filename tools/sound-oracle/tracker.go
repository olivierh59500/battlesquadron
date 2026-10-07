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

// generateTrackers fingerprints the source kind-eight movement before drawing.
// Only the offline oracle executes the original CPU; the game remains native Go.
func generateTrackers(assets, output string) {
	loader, err := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if err != nil {
		panic(err)
	}
	var references []controllerReference
	for stage := 0; stage < 2; stage++ {
		for _, point := range [][2]int{{40, -32}, {160, -32}, {100, 0}, {200, 120}} {
			b := &bus{memory: make([]byte, 1<<24)}
			copy(b.memory[0x100:], loader)
			b.Write16(0x9b84, 304)
			b.Write16(0x9b96, 1)
			b.Write16(0x9b9c, uint16(stage))
			b.Write32(0x7862, 0x20000)
			b.Write32(0x7866, 0x2000)
			b.Write8(0x76cb, 50)
			for index, x := range []int{112, 160} {
				player := uint32(0x4da2 + index*266)
				b.Write16(player+4, uint16(x+304))
				b.Write16(player+6, 176+256)
				b.Write8(player+38, 10)
			}
			b.Write32(0x2dc80, uint32((point[0]+304)<<16))
			b.Write32(0x2dc84, uint32((point[1]+256)<<16))
			b.Write8(0x2dc9f, 8)
			b.Write8(0x2dc98, 10)
			b.Write8(0x2dc9c, 200)
			cpu := m68k.New(b)
			var trajectory []byte
			ticks := 0
			for tick := 0; tick < 240; tick++ {
				b.Write16(0x1058, uint16(2*(tick+1)))
				registers := m68k.Registers{PC: 0x85c4, SR: 0x2700, SSP: 0x70000}
				registers.A[7], registers.A[4], registers.A[5] = 0x70000, 0x2dc80, 0x8000
				cpu.SetState(registers)
				for step := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x9774 && cpu.Registers().PC != 0x9842; step++ {
					if step > 10000 {
						panic(fmt.Sprintf("kind-eight controller did not return at $%x", cpu.Registers().PC))
					}
					if cpu.Registers().PC == 0x5aa4 {
						// Drawing is outside the offline movement boundary.
						registers := cpu.Registers()
						registers.PC = b.Read32(registers.A[7])
						registers.A[7] += 4
						registers.SSP = registers.A[7]
						cpu.SetState(registers)
						continue
					}
					if cpu.Step() == 0 {
						panic(fmt.Sprintf("kind-eight controller stopped at $%x", cpu.Registers().PC))
					}
				}
				if cpu.Registers().PC == 0x9842 {
					break
				}
				state := make([]byte, 20)
				binary.BigEndian.PutUint32(state, b.Read32(0x2dc80)-304<<16)
				binary.BigEndian.PutUint32(state[4:], b.Read32(0x2dc84)-256<<16)
				binary.BigEndian.PutUint32(state[8:], b.Read32(0x2dc88))
				binary.BigEndian.PutUint32(state[12:], b.Read32(0x2dc8c))
				state[16], state[17], state[18], state[19] = b.Read8(0x2dcbf), b.Read8(0x2dc9c), b.Read8(0x2dc9e)&3, b.Read8(0x2dc9b)
				trajectory = append(trajectory, state...)
				ticks++
			}
			traceDirectory := filepath.Join(filepath.Dir(output), "..", "..", ".cache", "tracker-traces")
			if err := os.MkdirAll(traceDirectory, 0755); err != nil {
				panic(err)
			}
			traceName := fmt.Sprintf("tracker_%d_%d_%d.bin", stage, point[0], point[1])
			if err := os.WriteFile(filepath.Join(traceDirectory, traceName), trajectory, 0644); err != nil {
				panic(err)
			}
			references = append(references, controllerReference{Kind: 8, Stage: stage, X: point[0], Y: point[1], Ticks: ticks, Hash: fmt.Sprintf("%x", sha256.Sum256(trajectory))})
		}
	}
	data, err := json.MarshalIndent(references, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %d original kind-eight movement fingerprints.\n", len(references))
}
