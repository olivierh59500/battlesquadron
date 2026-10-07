package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	m68k "github.com/user-none/go-chip-m68k"
)

func generateControls(assets, output string) {
	data, e := os.ReadFile(filepath.Join(assets, "lodgam.bin"))
	if e != nil {
		panic(e)
	}
	b := &bus{memory: make([]byte, 1<<24)}
	copy(b.memory[0x246f0:], data)
	cpu := m68k.New(b)
	call := func(address uint32, d0 uint32) {
		b.Write32(0x70000, 0x70010)
		r := m68k.Registers{PC: address, SR: 0x2700, SSP: 0x70000}
		r.A[7] = 0x70000
		r.D[0] = d0
		cpu.SetState(r)
		for step := 0; cpu.Registers().PC != 0x70010; step++ {
			if step > 1000000 || cpu.Step() == 0 {
				panic("original sound control did not return")
			}
		}
	}
	call(0x24c6e, 0)
	references := []reference{}
	checkpoints := map[int]bool{0: true, 1: true, 2: true, 8: true, 16: true, 64: true, 128: true, 256: true, 512: true, 1024: true, 4096: true}
	for tick := 0; tick <= 4096; tick++ {
		if checkpoints[tick] {
			state := append([]byte(nil), b.memory[0x251f8:0x25204]...)
			state = append(state, b.Read8(0x251f6))
			state = append(state, b.memory[0x252a4:0x2539c]...)
			for i := uint32(0); i < 4; i++ {
				state = append(state, b.memory[0xdff0a0+i*16:0xdff0aa+i*16]...)
				enabled := byte(0)
				if b.dma&(1<<i) != 0 {
					enabled = 1
				}
				state = append(state, enabled)
			}
			waveforms := append([]byte(nil), b.memory[0x246f0:0x246f0+uint32(len(data))]...)
			clear(waveforms[0x251f6-0x246f0 : 0x25204-0x246f0])
			clear(waveforms[0x252a4-0x246f0 : 0x2539c-0x246f0])
			clear(waveforms[0x24e34-0x246f0 : 0x24f34-0x246f0])
			references = append(references, reference{"lodgam", 1, tick, fmt.Sprintf("%x", sha256.Sum256(state)), fmt.Sprintf("%x", sha256.Sum256(waveforms))})
		}
		switch tick {
		case 42:
			call(0x24d6e, 17)
		case 100, 200:
			call(0x24cfc, 0)
		case 160:
			call(0x24d6e, 28)
		case 300, 400:
			call(0x24d06, 0)
		case 320:
			call(0x24d6e, 28)
		case 560:
			call(0x24d6e, 57)
		}
		call(0x24f34, 0)
	}
	encoded, e := json.MarshalIndent(references, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(output, append(encoded, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Printf("Generated %d original sound-control fingerprints.\n", len(references))
}
