package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	m68k "github.com/user-none/go-chip-m68k"
)

type allocationReference struct {
	Kind, Players, TemplateHealth int
	Boundary                      bool
	Hash                          string
}

// generateAllocations executes the supplied loader's actual $7544 allocation
// through its armor branch at $75CA. The game's two-player flag is operand
// $F4B4(A5), whose signed displacement addresses RAM $74B4 when A5 is $8000.
func generateAllocations(assets, output string) {
	loader, err := os.ReadFile(filepath.Join(assets, "loader.bin"))
	if err != nil {
		panic(err)
	}
	var refs []allocationReference
	for kind := 0; kind < 14; kind++ {
		health := int(loader[0xcce0-0x100+kind*32+12])
		for players := 1; players <= 2; players++ {
			refs = append(refs, allocationReference{Kind: kind, Players: players, TemplateHealth: health})
		}
	}
	for _, health := range []int{0, 1, 3, 255} {
		for players := 1; players <= 2; players++ {
			refs = append(refs, allocationReference{Kind: 6, Players: players, TemplateHealth: health, Boundary: true})
		}
	}
	for index := range refs {
		ref := &refs[index]
		b := &bus{memory: make([]byte, 1<<24)}
		copy(b.memory[0x100:], loader)
		b.Write8(0xcce0+uint32(ref.Kind)*32+12, byte(ref.TemplateHealth))
		if ref.Players == 2 {
			b.Write8(0x74b4, 255)
		} else {
			b.Write8(0x74b4, 0)
		}
		b.Write8(0x406e, 0)
		cpu := m68k.New(b)
		r := m68k.Registers{PC: 0x7544, SR: 0x2700, SSP: 0x70000}
		r.A[5], r.A[7] = 0x8000, 0x70000
		r.D[1], r.D[2], r.D[3] = 128, 0xffffffd4, uint32(ref.Kind)
		cpu.SetState(r)
		for steps := 0; cpu.Registers().PC != 0x75ca; steps++ {
			if steps > 10000 || cpu.Step() == 0 {
				panic(fmt.Sprintf("original flying allocation stuck at$%x", cpu.Registers().PC))
			}
		}
		ref.Hash = fmt.Sprintf("%x", sha256.Sum256([]byte{b.Read8(cpu.Registers().A[0] + 24)}))
	}
	data, err := json.MarshalIndent(refs, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0644); err != nil {
		panic(err)
	}
	fmt.Printf("Generated %d original flying-allocation fingerprints.\n", len(refs))
}
