// This offline oracle fingerprints the original sound routines. The game never
// imports this module or its 68000 dependency; runtime playback is native Go.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	m68k "github.com/user-none/go-chip-m68k"
)

type bus struct {
	memory []byte
	dma    uint16
}

func (b *bus) Reset()                  {}
func (b *bus) Read8(a uint32) byte     { return b.memory[a] }
func (b *bus) Read16(a uint32) uint16  { return binary.BigEndian.Uint16(b.memory[a:]) }
func (b *bus) Read32(a uint32) uint32  { return binary.BigEndian.Uint32(b.memory[a:]) }
func (b *bus) Write8(a uint32, v byte) { b.memory[a] = v }
func (b *bus) Write16(a uint32, v uint16) {
	if a == 0xdff096 {
		if v&0x8000 != 0 {
			b.dma |= v & 15
		} else {
			b.dma &^= v & 15
		}
	}
	binary.BigEndian.PutUint16(b.memory[a:], v)
}
func (b *bus) Write32(a uint32, v uint32) { binary.BigEndian.PutUint32(b.memory[a:], v) }

type overlay struct {
	name                                                       string
	base, init, interrupt, selectTrack, effect, state, channel uint32
	tracks                                                     int
	globalLength                                               int
}
type reference struct {
	Overlay     string
	Track, Tick int
	Hash        string
	DataHash    string
}

var overlays = []overlay{
	{"lodgam", 0x246f0, 0x24c6e, 0x24f34, 0x24dde, 0x24d6e, 0x251f8, 0x252a4, 10, 12},
	{"lodmus", 0x3d800, 0x3d800, 0x3dda2, 0x3dd7e, 0x3dd22, 0x3df16, 0x3df3c, 1, 6},
	{"lodcom", 0x3d800, 0x3dc64, 0x3dd74, 0x3dd50, 0x3dcf4, 0x3dee8, 0x3defe, 1, 6},
}

func main() {
	pickups := flag.Bool("pickups", false, "fingerprint the original collectable capsule movement")
	aim := flag.Bool("aim", false, "record the original aimed enemy-projectile arithmetic")
	waves := flag.Bool("waves", false, "fingerprint original directed flying formations instead of sound")
	controllers := flag.Bool("controllers", false, "fingerprint original adaptive flying controllers instead of sound")
	trackers := flag.Bool("trackers", false, "fingerprint original kind-eight tracking controllers instead of sound")
	assets := flag.String("assets", "../../assets/unpacked", "extracted original sound overlays")
	output := flag.String("out", "../../internal/sound/oracle_reference_test.json", "native regression fingerprints")
	dump := flag.String("dump", "../../.cache/sound-oracle/dumps", "original state dumps for development comparison")
	flag.Parse()
	if *pickups {
		generatePickups(*assets, *output)
		return
	}
	if *aim {
		generateAim(*assets, *output)
		return
	}
	if *trackers {
		generateTrackers(*assets, *output)
		return
	}
	if *controllers {
		generateControllers(*assets, *output)
		return
	}
	if *waves {
		generateWaves(*assets, *output)
		return
	}
	checkpoints := map[int]bool{0: true, 1: true, 2: true, 4: true, 8: true, 16: true, 64: true, 256: true, 1024: true, 4096: true}
	references := []reference{}
	for _, o := range overlays {
		data, e := os.ReadFile(filepath.Join(*assets, o.name+".bin"))
		if e != nil {
			panic(e)
		}
		for track := 1; track <= o.tracks; track++ {
			b := &bus{memory: make([]byte, 1<<24)}
			copy(b.memory[o.base:], data)
			cpu := m68k.New(b)
			call := func(address uint32, d0 uint32) {
				b.Write32(0x70000, 0x70010)
				r := m68k.Registers{PC: address, SR: 0x2700, SSP: 0x70000}
				r.A[7] = 0x70000
				r.D[0] = d0
				cpu.SetState(r)
				for step := 0; cpu.Registers().PC != 0x70010; step++ {
					if step > 1000000 || cpu.Step() == 0 {
						panic(fmt.Sprintf("original %s routine $%X did not return at $%X", o.name, address, cpu.Registers().PC))
					}
				}
			}
			call(o.init, 0)
			if track != 1 {
				call(o.selectTrack, uint32(track))
			}
			for tick := 0; tick <= 4096; tick++ {
				if checkpoints[tick] {
					state := append([]byte(nil), b.memory[o.state:o.state+uint32(o.globalLength)]...)
					state = append(state, b.Read8(o.state-2))
					state = append(state, b.memory[o.channel:o.channel+248]...)
					for i := uint32(0); i < 4; i++ {
						state = append(state, b.memory[0xdff0a0+i*16:0xdff0aa+i*16]...)
						enabled := byte(0)
						if b.dma&(1<<i) != 0 {
							enabled = 1
						}
						state = append(state, enabled)
					}
					waveforms := append([]byte(nil), b.memory[o.base:o.base+uint32(len(data))]...)
					clear(waveforms[o.state-o.base-2 : o.state-o.base+uint32(o.globalLength)])
					clear(waveforms[o.channel-o.base : o.channel-o.base+248])
					if o.name == "lodgam" {
						clear(waveforms[0x24e34-o.base : 0x24f34-o.base])
					}
					references = append(references, reference{o.name, track, tick, fmt.Sprintf("%x", sha256.Sum256(state)), fmt.Sprintf("%x", sha256.Sum256(waveforms))})
					if *dump != "" {
						if e = os.MkdirAll(*dump, 0755); e != nil {
							panic(e)
						}
						if e = os.WriteFile(filepath.Join(*dump, fmt.Sprintf("%s-%d-%d.bin", o.name, track, tick)), state, 0644); e != nil {
							panic(e)
						}
					}
				}
				if o.name == "lodgam" {
					switch tick {
					case 42:
						call(o.effect, 17)
					case 160:
						call(o.effect, 28)
					case 560:
						call(o.effect, 57)
					}
				}
				call(o.interrupt, 0)
			}
		}
	}
	data, e := json.MarshalIndent(references, "", "  ")
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile(*output, append(data, '\n'), 0644); e != nil {
		panic(e)
	}
	fmt.Printf("Generated %d original sound-state fingerprints.\n", len(references))
}
