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

type caveBossReference struct {
	Mode, Ticks int
	Hash        string
	Damage      bool
}

// generateCaveBosses records both original kind-nine multipart controllers.
func generateCaveBosses(assets, output string) {
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(assets, name+".bin"))
		if err != nil {
			panic(err)
		}
		return data
	}
	loader, resident, first := read("loader"), read("loddat"), read("lodst1")
	var references []caveBossReference
	for mode := 1; mode <= 2; mode++ {
		for _, damaged := range []bool{false, true} {
			b := &bus{memory: make([]byte, 1<<24)}
			copy(b.memory[0x100:], loader)
			copy(b.memory[0x10000:], resident)
			copy(b.memory[0x2e89a:], first)
			b.Write16(0x9b84, 304)
			b.Write16(0x9b9c, uint16(mode))
			b.Write16(0x9b96, 1)
			progress, y := 2601, -80
			if mode == 2 {
				progress, y = 7701, -87
			}
			b.Write16(0x9b86, uint16(progress))
			for index, x := range []int{112, 160} {
				player := uint32(0x4da2 + index*266)
				b.Write16(player+4, uint16(x+304))
				b.Write16(player+6, 176+256)
				b.Write8(player+38, 0)
			}
			for slot := 0; slot < 2; slot++ {
				record := uint32(0x2df00 + slot*80)
				partY := y
				if slot == 1 {
					if mode == 1 {
						partY += 32
					} else {
						partY += 48
					}
				}
				b.Write32(record, uint32((96+304)<<16))
				b.Write32(record+4, uint32((partY+256)<<16))
				b.Write8(record+31, 9)
				b.Write8(record+24, 127)
				b.Write16(record+50, 32)
				b.Write16(record+68, 96)
			}
			cpu := m68k.New(b)
			var trajectory []byte
			ticks := 0
			for tick := 0; tick < 600; tick++ {
				step := tick + 1
				if damaged {
					if step == 40 || step == 80 && mode == 1 {
						b.Write8(0x2df50+62, 128)
					}
					if step == 120 || step == 160 {
						b.Write8(0x2df00+62, 128)
					}
					if mode == 2 && (step == 200 || step == 300) {
						b.Write8(0x2df00+62, 128)
					}
				}
				b.Write16(0x1058, uint16(2*(tick+1)))
				var frames [2]byte
				removed := false
				for slot := 0; slot < 2; slot++ {
					r := m68k.Registers{PC: 0x7f24, SR: 0x2700, SSP: 0x70000}
					r.A[7], r.A[4], r.A[5] = 0x70000, uint32(0x2df00+slot*80), 0x8000
					cpu.SetState(r)
					for steps := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x977c && cpu.Registers().PC != 0x9842; steps++ {
						if steps > 10000 {
							panic(fmt.Sprintf("cave%d part%d stuck at$%x", mode, slot, cpu.Registers().PC))
						}
						if cpu.Registers().PC == 0x2470e {
							r := cpu.Registers()
							r.PC = b.Read32(r.A[7])
							r.A[7] += 4
							r.SSP = r.A[7]
							cpu.SetState(r)
							continue
						}
						if cpu.Step() == 0 {
							panic(fmt.Sprintf("cave%d part%d stopped at$%x", mode, slot, cpu.Registers().PC))
						}
					}
					if cpu.Registers().PC == 0x9842 {
						removed = true
						break
					}
					frames[slot] = b.Read8(uint32(0x2df00 + slot*80 + 63))
					if mode == 2 && slot == 1 {
						frames[slot] = 0
						if cpu.Registers().A[2] == 0x365d8 {
							frames[slot] = 2
						}
						if cpu.Registers().A[2] == 0x36bf0 {
							frames[slot] = 3
						}
					}
					if mode == 2 && slot == 0 {
						frames[slot] = byte((cpu.Registers().A[2] - 0x2e4c0) / 0xf00)
					}
				}
				if removed {
					break
				}
				state := make([]byte, 45)
				for slot := 0; slot < 2; slot++ {
					record := uint32(0x2df00 + slot*80)
					destination := state[slot*22:]
					binary.BigEndian.PutUint32(destination, b.Read32(record)-304<<16)
					binary.BigEndian.PutUint32(destination[4:], b.Read32(record+4)-256<<16)
					binary.BigEndian.PutUint32(destination[8:], b.Read32(record+8))
					destination[12], destination[13], destination[14], destination[15], destination[16] = frames[slot], b.Read8(record+24), b.Read8(record+27), b.Read8(record+28), b.Read8(record+57)
					destination[17] = b.Read8(record+30) & 128
					if mode == 1 && slot == 0 {
						binary.BigEndian.PutUint32(destination[18:], uint32(b.Read16(record+12))/4)
					}
				}
				state[44] = b.Read8(0x2aa5)
				trajectory = append(trajectory, state...)
				ticks++
			}
			references = append(references, caveBossReference{Mode: mode, Ticks: ticks, Hash: fmt.Sprintf("%x", sha256.Sum256(trajectory)), Damage: damaged})
			traceDir := filepath.Join(filepath.Dir(output), "..", "..", ".cache", "boss-traces")
			if err := os.MkdirAll(traceDir, 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(filepath.Join(traceDir, fmt.Sprintf("cave_%d_%v.bin", mode, damaged)), trajectory, 0644); err != nil {
				panic(err)
			}
		}
	}
	data, err := json.MarshalIndent(references, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated original cave-boss movement fingerprints.")
}
