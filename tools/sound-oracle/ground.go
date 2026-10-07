package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	m68k "github.com/user-none/go-chip-m68k"
	"os"
	"path/filepath"
)

type groundReference struct {
	Kind, Mode, Template, X, Y, Ticks int
	Damage                            bool
	Hash, Trace                       string
}

// generateGround runs original scenery instructions only in this offline module.
func generateGround(assets, output string) {
	read := func(name string) []byte {
		d, e := os.ReadFile(filepath.Join(assets, name+".bin"))
		if e != nil {
			panic(e)
		}
		return d
	}
	loader, dat := read("loader"), read("loddat")
	refs := []groundReference{}
	cases := [][3]int{{1, 0, 0x2b20}, {32, 0, 0x2d30}, {32, 1, 0x2d60}, {32, 2, 0x2d90}, {32, 3, 0x2dc0}, {34, 0, 0x2e50}, {34, 2, 0x2e20}, {37, 0, 0x2ee0}, {40, 1, 0x2f70}}
	for _, test := range cases {
		kind, mode, template := test[0], test[1], test[2]
		for _, damage := range []bool{false, true} {
			b := &bus{memory: make([]byte, 1<<24)}
			copy(b.memory[0x100:], loader)
			copy(b.memory[0x10000:], dat)
			b.Write16(0x9b84, 304)
			b.Write16(0x9b96, 1)
			b.Write16(0x9b9c, uint16(mode))
			b.Write8(0x74bc, 255)
			for i, x := range []int{112, 160} {
				p := uint32(0x4da2 + i*266)
				b.Write16(p+4, uint16(x+304))
				b.Write16(p+6, 176+256)
				b.Write8(p+38, 0)
			}
			for off := 0x1000; off < 0x1250; off += 2 {
				if bytes.Equal(loader[off:off+4], []byte{0x1b, 0x7c, 0, 10}) {
					address := uint32(0x8000 + int(int16(binary.BigEndian.Uint16(loader[off+4:]))))
					b.Write8(address, 10)
					b.Write8(address+1, 50)
				}
			}
			record := uint32(0x2e040)
			copy(b.memory[record:record+48], b.memory[template:template+48])
			b.Write16(record, 404)
			b.Write16(record+2, 256)
			width, height := int(b.Read16(record+8)), int(b.Read16(record+6))
			b.Write16(record+10, uint16(width*2*height))
			b.Write16(record+26, uint16(width*2*height*5))
			b.Write8(record+43, 1)
			if int(b.Read8(record+17)) != kind {
				panic(fmt.Sprintf("template%x kind%d expected%d", template, b.Read8(record+17), kind))
			}
			cpu := m68k.New(b)
			trajectory := []byte{}
			for tick := 0; tick < 80; tick++ {
				if damage && tick == 20 {
					b.Write8(record+24, 128)
				}
				b.Write16(0x1058, uint16(2*tick))
				r := m68k.Registers{PC: 0x5e9a, SR: 0x2700, SSP: 0x70000}
				r.A[7], r.A[5], r.A[6] = 0x70000, 0x8000, 0xdff000
				cpu.SetState(r)
				for step := 0; cpu.Registers().PC != 0x6e44 && cpu.Registers().PC != 0x6f4c; step++ {
					if step > 50000 {
						panic(fmt.Sprintf("ground%d stuck%x", kind, cpu.Registers().PC))
					}
					pc := cpu.Registers().PC
					if pc == 0x2470e || pc == 0x7544 {
						r := cpu.Registers()
						r.PC = b.Read32(r.A[7])
						r.A[7] += 4
						r.SSP = r.A[7]
						r.D[0] = 0
						cpu.SetState(r)
						continue
					}
					if cpu.Step() == 0 {
						panic(fmt.Sprintf("ground%d stopped%x", kind, cpu.Registers().PC))
					}
				}
				state := make([]byte, 14)
				binary.BigEndian.PutUint16(state, b.Read16(record)-304)
				binary.BigEndian.PutUint16(state[2:], b.Read16(record+2)-256)
				for i, off := range []uint32{25, 31, 18, 28, 32, 30, 36, 37, 43, 29} {
					state[4+i] = b.Read8(record + off)
				}
				state[5] &= 30
				trajectory = append(trajectory, state...)
			}
			trace := fmt.Sprintf("ground_%d_%d_%v.bin", kind, mode, damage)
			dir := filepath.Join(filepath.Dir(output), "../../.cache/ground-traces")
			os.MkdirAll(dir, 0755)
			os.WriteFile(filepath.Join(dir, trace), trajectory, 0644)
			refs = append(refs, groundReference{kind, mode, template, 100, 0, 80, damage, fmt.Sprintf("%x", sha256.Sum256(trajectory)), trace})
		}
	}
	data, e := json.MarshalIndent(refs, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(output, append(data, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Println("Generated original ground-controller fingerprints.")
}
