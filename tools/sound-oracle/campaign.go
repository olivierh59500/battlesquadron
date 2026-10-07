package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	m68k "github.com/user-none/go-chip-m68k"
)

type campaignReference struct {
	Name                      string
	Phase, Mask, Score, Bonus int
	Ticks                     int
	Hash                      string
}

// generateCampaign executes actual loader routines at base $100. Hardware sound
// calls are skipped at their entry; framebuffer writes are allowed but omitted
// from the hashes. This is an independent campaign-seam check, not a complete
// emulator or a matched-input reference game.
func generateCampaign(assets, output string) {
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join(assets, name+".bin"))
		if err != nil {
			panic(err)
		}
		return data
	}
	loader, resident, surface := read("loader"), read("loddat"), read("lods0t")
	caves := [][]byte{nil, read("lodst1"), read("lodst2"), read("lodst3")}
	newFixture := func() (*bus, *m68k.CPU) {
		b := &bus{memory: make([]byte, 1<<24)}
		copy(b.memory[0x100:], loader)
		copy(b.memory[0x10000:], resident)
		copy(b.memory[0x44000:], surface)
		b.Write32(0x9b98, 0x148e)
		b.Write32(0x2aa2, 0x17400)
		b.Write16(0x9b84, 304)
		b.Write32(0x9b88, 0x65000)
		b.Write16(0x9b8c, 0)
		b.Write16(0x9b92, 256)
		b.Write16(0xa0ac, 0)
		b.Write8(0x793e, 0)
		for index, x := range []int{112, 160} {
			p := uint32(0x4da2 + 266*index)
			b.Write16(p, uint16(x+256))
			b.Write16(p+2, 336)
			b.Write16(p+10, 6)
			b.Write8(p+38, 0)
			b.Write8(p+39, 7)
			b.Write8(p+49, 0)
			b.Write16(p+54, uint16(x+256))
			b.Write8(p+56, 3)
			b.Write16(p+58, 2)
			b.Write16(p+60, 3)
			b.Write16(p+66, 4)
			copy(b.memory[p+106:p+114], "00123456")
		}
		return b, m68k.New(b)
	}
	stepUntil := func(b *bus, cpu *m68k.CPU, start, end uint32, a3 uint32) {
		b.Write32(0x70000, 0x70010)
		r := m68k.Registers{PC: start, SR: 0x2700, SSP: 0x70000}
		r.A[3], r.A[5], r.A[6], r.A[7] = a3, 0x8000, 0xdff000, 0x70000
		cpu.SetState(r)
		for steps := 0; cpu.Registers().PC != end; steps++ {
			if steps > 100000 {
				panic(fmt.Sprintf("campaign routine$%x stuck at$%x", start, cpu.Registers().PC))
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
				panic(fmt.Sprintf("campaign routine$%x stopped at$%x", start, cpu.Registers().PC))
			}
		}
	}
	appendInt := func(state []byte, values ...int) []byte {
		for _, value := range values {
			state = binary.BigEndian.AppendUint32(state, uint32(value))
		}
		return state
	}
	transitionState := func(b *bus) []byte {
		state := appendInt(nil, int(b.Read16(0x9b86)), int(b.Read16(0x9b84))-256, int(b.Read8(0x6f5d)))
		for index := 0; index < 2; index++ {
			p := uint32(0x4da2 + 266*index)
			state = appendInt(state, int(b.Read16(p))-256, int(b.Read16(p+2))-256, int(b.Read16(p+10))/2, int(b.Read16(p+52)), int(b.Read8(p+48)), int(b.Read8(p+56)), int(b.Read16(p+58)), int(b.Read16(p+60)), int(b.Read16(p+66)))
			score := 0
			for _, digit := range b.memory[p+106 : p+114] {
				score = score*10 + int(digit-'0')
			}
			state = appendInt(state, score)
		}
		return state
	}
	bonusState := func(b *bus) []byte {
		state := []byte{}
		for index := 0; index < 2; index++ {
			p := uint32(0x4da2 + 266*index)
			score := 0
			for _, digit := range b.memory[p+106 : p+114] {
				score = score*10 + int(digit-'0')
			}
			remaining, awarded := b.Read8(p+97), b.Read8(p+98)
			state = appendInt(state, score, int(remaining>>4)*10+int(remaining&15), int(awarded>>4)*10+int(awarded&15))
		}
		return state
	}
	var refs []campaignReference
	for phase := 1; phase <= 3; phase++ {
		for _, returning := range []bool{false, true} {
			b, cpu := newFixture()
			mode, previous := phase, 0
			name := "prefill-entry"
			if returning {
				mode, previous, name = 0, phase, "prefill-return"
			} else {
				copy(b.memory[[]int{0, 0x2e89a, 0x2e4c0, 0x2e840}[phase]:], caves[phase])
			}
			b.Write16(0x9b9c, uint16(mode))
			b.Write16(0x9ba0, uint16(previous))
			b.Write8(0x6f5d, 0)
			b.Write8(0x74b4, 255)
			b.Write8(0x76cc, 100)
			// The stage initializer clears pools and restores map/camera cursors.
			stepUntil(b, cpu, 0x11a6, 0x70010, 0)
			stepUntil(b, cpu, 0x71c4, 0x722c, 0)
			stepUntil(b, cpu, 0x7448, 0x70010, 0x4da2)
			stepUntil(b, cpu, 0x7448, 0x70010, 0x4eac)
			for tick := 0; tick < 256; tick++ {
				stepUntil(b, cpu, 0x9ba4, 0x70010, 0)
				stepUntil(b, cpu, 0x5e9a, 0x70010, 0)
				stepUntil(b, cpu, 0x3000, 0x70010, 0)
			}
			var objects [][]byte
			for slot := 0; slot < 18; slot++ {
				record := uint32(0x2e040 + slot*64)
				if b.Read16(record) == 0 {
					continue
				}
				state := []byte{b.Read8(record + 17)}
				state = binary.BigEndian.AppendUint16(state, b.Read16(record)-b.Read16(0x9b84))
				state = binary.BigEndian.AppendUint16(state, b.Read16(record+2)-256)
				state = append(state, b.Read8(record+25), b.Read8(record+31)&30, b.Read8(record+18), b.Read8(record+28), b.Read8(record+32), b.Read8(record+30), b.Read8(record+36), b.Read8(record+37), b.Read8(record+43), b.Read8(record+42))
				objects = append(objects, state)
			}
			slices.SortFunc(objects, bytes.Compare)
			state := appendInt(nil, int(b.Read16(0x9b86)), int(b.Read16(0x9b84))-256, int(b.Read32(0x2aa2)&255), len(objects))
			for _, object := range objects {
				state = append(state, object...)
			}
			traceDir := filepath.Join(filepath.Dir(output), "..", "..", ".cache", "campaign-traces")
			if err := os.MkdirAll(traceDir, 0755); err != nil {
				panic(err)
			}
			if err := os.WriteFile(filepath.Join(traceDir, fmt.Sprintf("%s-%d.bin", name, phase)), state, 0644); err != nil {
				panic(err)
			}
			refs = append(refs, campaignReference{Name: name, Phase: phase, Ticks: 256, Hash: fmt.Sprintf("%x", sha256.Sum256(state))})
		}
	}
	for phase := 0; phase <= 3; phase++ {
		for _, mask := range []int{0, 14} {
			b, cpu := newFixture()
			b.Write16(0x9b9c, uint16(phase))
			b.Write8(0x6f5d, byte(mask))
			b.Write16(0x9b86, 8192)
			b.Write32(0x9b8e, 0x44000)
			b.Write16(0x9b8c, 0)
			b.Write16(0x9ba2, 0)
			b.Write8(0x6f5c, 0)
			var trace []byte
			ticks := 1
			if phase == 3 {
				ticks += 300
			}
			for tick := 0; tick < ticks; tick++ {
				stepUntil(b, cpu, 0x9ba4, 0x70010, 0)
				trace = appendInt(trace, int(b.Read16(0x9b86)), int(b.Read16(0x9ba2)), int(b.Read8(0x6f5c)))
			}
			refs = append(refs, campaignReference{Name: "map-end", Phase: phase, Mask: mask, Ticks: ticks, Hash: fmt.Sprintf("%x", sha256.Sum256(trace))})
		}
	}
	for phase := 0; phase <= 3; phase++ {
		for _, mask := range []int{0, 14 &^ (1 << uint(phase))} {
			if phase == 0 && mask == 14 {
				// A completed campaign cannot enter another cave.
				continue
			}
			b, cpu := newFixture()
			b.Write16(0x9ba0, uint16(phase))
			b.Write8(0x6f5d, byte(mask))
			stepUntil(b, cpu, 0x71c4, 0x722c, 0)
			stepUntil(b, cpu, 0x7448, 0x70010, 0x4da2)
			stepUntil(b, cpu, 0x7448, 0x70010, 0x4eac)
			for tick := 0; tick < 256; tick++ {
				stepUntil(b, cpu, 0x9ba4, 0x70010, 0)
			}
			refs = append(refs, campaignReference{Name: "transition", Phase: phase, Mask: mask, Score: 123456, Ticks: 256, Hash: fmt.Sprintf("%x", sha256.Sum256(transitionState(b)))})
		}
	}
	for _, score := range []int{0, 99000, 99999000} {
		for _, bonus := range []int{0, 1, 10, 99} {
			for _, disabled := range []bool{false, true} {
				b, cpu := newFixture()
				for index := 0; index < 2; index++ {
					p := uint32(0x4da2 + 266*index)
					copy(b.memory[p+106:p+114], fmt.Sprintf("%08d", score))
					b.Write8(p+97, byte(bonus/10<<4|bonus%10))
					b.Write8(p+98, 0)
					if disabled && index == 1 {
						b.Write8(p+38, 255)
						b.Write8(p+97, 0)
					}
				}
				var trace []byte
				for clock := 0; clock <= 360; clock++ {
					b.Write16(0x1058, uint16(clock))
					stepUntil(b, cpu, 0x734a, 0x70010, 0)
					trace = append(trace, bonusState(b)...)
				}
				mask := 0
				if disabled {
					mask = 2
				}
				refs = append(refs, campaignReference{Name: "wreck", Mask: mask, Score: score, Bonus: bonus, Ticks: 361, Hash: fmt.Sprintf("%x", sha256.Sum256(trace))})
			}
		}
	}
	for _, score := range []int{0, 99900000} {
		for _, disabled := range []bool{false, true} {
			b, cpu := newFixture()
			for index := 0; index < 2; index++ {
				p := uint32(0x4da2 + 266*index)
				copy(b.memory[p+106:p+114], fmt.Sprintf("%08d", score))
				b.Write8(p+97, 0)
				b.Write8(p+98, 0)
			}
			if disabled {
				b.Write8(0x4eac+38, 255)
			}
			var trace []byte
			for clock := 0; clock < 100; clock++ {
				stepUntil(b, cpu, 0xe10, 0xe40, 0)
				trace = append(trace, bonusState(b)...)
			}
			mask := 0
			if disabled {
				mask = 2
			}
			refs = append(refs, campaignReference{Name: "ending", Mask: mask, Score: score, Ticks: 100, Hash: fmt.Sprintf("%x", sha256.Sum256(trace))})
		}
	}
	data, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %d original campaign and bonus fingerprints.\n", len(refs))
}
