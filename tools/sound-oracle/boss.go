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

type bossReference struct {
	Name  string
	Ticks int
	Hash  string
}

// generateBosses records the original four-part final-boss update boundary.
func generateBosses(assets, output string) {
	loader, err := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if err != nil {
		panic(err)
	}
	resident, err := os.ReadFile(filepath.Join(assets, "loddat.bin"))
	if err != nil {
		panic(err)
	}
	var references []bossReference
	for _, name := range []string{"final-entry", "final-damage"} {
		b := &bus{memory: make([]byte, 1<<24)}
		copy(b.memory[0x100:], loader)
		copy(b.memory[0x10000:], resident)
		b.Write16(0x9b84, 304)
		b.Write16(0x9b86, 240)
		b.Write16(0x9b96, 0)
		b.Write8(0x1922, 255)
		b.Write16(0x7940, 0)
		for slot, point := range [][2]int{{90, -78}, {10, -40}, {186, -40}, {90, -50}} {
			record := uint32(0x2df00 + slot*80)
			b.Write32(record, uint32((point[0]+304)<<16))
			b.Write32(record+4, uint32((point[1]+256)<<16))
			b.Write8(record+31, 2)
			b.Write16(record+24, b.Read16(0xcd20+12))
			b.Write8(record+27, b.Read8(0xcd20+26))
			b.Write8(record+28, b.Read8(0xcd20+27))
			b.Write16(record+50, 32)
			b.Write16(record+68, 80)
		}
		cpu := m68k.New(b)
		var trajectory []byte
		ticks := 0
		maximumTicks := 256
		if name == "final-damage" {
			maximumTicks = 900
		}
		for tick := 0; tick < maximumTicks; tick++ {
			step := tick + 1
			if name == "final-damage" {
				switch step {
				case 40:
					b.Write8(0x2df50+62, 90)
					b.Write8(0x2dfa0+62, 90)
				case 80:
					b.Write8(0x2dff0+62, 90)
				case 120:
					b.Write8(0x2dff0+62, 128)
				}
				if step >= 400 && step < 656 {
					b.Write8(0x2dff0+62, 1)
				}
			}
			b.Write16(0x1058, uint16(2*(tick+1)))
			var frames [4]byte
			for slot := 0; slot < 4; slot++ {
				registers := m68k.Registers{PC: 0x8ef2, SR: 0x2700, SSP: 0x70000}
				registers.A[7], registers.A[4], registers.A[5] = 0x70000, uint32(0x2df00+slot*80), 0x8000
				cpu.SetState(registers)
				for steps := 0; cpu.Registers().PC != 0x9758 && cpu.Registers().PC != 0x977c; steps++ {
					if steps > 10000 || cpu.Step() == 0 {
						panic(fmt.Sprintf("final-boss part%d stopped at $%x", slot, cpu.Registers().PC))
					}
				}
				frames[slot] = b.Read8(uint32(0x2df00 + slot*80 + 63))
				if slot == 0 {
					frames[slot] = byte((cpu.Registers().A[2] - 0x44000) / 0x690)
				}
				if slot == 3 {
					frames[slot] = byte((cpu.Registers().A[2] - 0x457a0) / 0xb40)
				}
			}
			state := make([]byte, 4*15+3)
			for slot := 0; slot < 4; slot++ {
				record := uint32(0x2df00 + slot*80)
				destination := state[slot*15:]
				binary.BigEndian.PutUint32(destination, b.Read32(record)-304<<16)
				binary.BigEndian.PutUint32(destination[4:], b.Read32(record+4)-256<<16)
				destination[8], destination[9], destination[10] = frames[slot], b.Read8(record+24), b.Read8(record+27)
				destination[11], destination[12], destination[13], destination[14] = b.Read8(record+28), b.Read8(record+26), b.Read8(record+57), 0
			}
			state[60], state[61], state[62] = b.Read8(0x7940), b.Read8(0x7941), b.Read8(0x2aa5)
			trajectory = append(trajectory, state...)
			ticks++
			if b.Read8(0x6f60) != 0 {
				break
			}
		}
		references = append(references, bossReference{Name: name, Ticks: ticks, Hash: fmt.Sprintf("%x", sha256.Sum256(trajectory))})
		traceDir := filepath.Join(filepath.Dir(output), "..", "..", ".cache", "boss-traces")
		if err := os.MkdirAll(traceDir, 0755); err != nil {
			panic(err)
		}
		if err := os.WriteFile(filepath.Join(traceDir, name+".bin"), trajectory, 0644); err != nil {
			panic(err)
		}
	}
	data, err := json.MarshalIndent(references, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Println("Generated original multipart final-boss entry fingerprint.")
}
