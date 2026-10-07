package source

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// Bundle lists derived original resources consumed by the native game.
type Bundle struct {
	Title   string
	Font    string
	Loader  string
	Sprites []Sprite
	Stages  []Stage
}

// Sprite describes a decoded original planar graphic.
type Sprite struct {
	ID, File, Kind        string
	Width, Height         int
	Address               uint32
	Graphic, Frame, Stage int
	TemplateAddress       uint32
}

// Stage describes the original 24-column terrain stream.
type Stage struct {
	ID, Mode                    int
	File, TilesFile             string
	Width, Height, Rows         int
	TileBankAddress, MapAddress uint32
	Tiles                       []uint16
}

// WriteGraphics reconstructs native PNG resources using original bitplane data.
func WriteGraphics(root string) error {
	read := func(name string) ([]byte, error) { return os.ReadFile(filepath.Join(root, "unpacked", name+".bin")) }
	loader, e := read("loader")
	if e != nil {
		return e
	}
	dat, e := read("loddat")
	if e != nil {
		return e
	}
	pal := make(color.Palette, 32)
	needle := []byte{1, 0x80, 0, 0, 1, 0x82, 6, 0x10, 1, 0x84, 0xd, 0xdd}
	p := bytes.Index(loader, needle)
	if p < 0 {
		return fmt.Errorf("original Copper palette not found")
	}
	for i := 0; i < 32; i++ {
		v := binary.BigEndian.Uint16(loader[p+i*4+2:])
		pal[i] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
	}
	bundle := Bundle{Title: "graphics/title.png", Font: "graphics/font.png", Loader: "unpacked/loader.bin"}
	save := func(file string, img image.Image) error {
		target := filepath.Join(root, file)
		if e := os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			return e
		}
		f, e := os.Create(target)
		if e != nil {
			return e
		}
		defer f.Close()
		return png.Encode(f, img)
	}
	planar := func(data []byte, start, w, h, planes, stride, rowBytes int, transparent bool) *image.NRGBA {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				v := 0
				for plane := 0; plane < planes; plane++ {
					off := start + plane*stride + y*rowBytes + x/8
					if off >= 0 && off < len(data) {
						v |= int((data[off]>>uint(7-x%8))&1) << plane
					}
				}
				if transparent && v == 0 {
					continue
				}
				img.Set(x, y, pal[v%len(pal)])
			}
		}
		return img
	}
	// The original score font stores one eight-pixel row per byte and ten bytes per glyph.
	font := image.NewNRGBA(image.Rect(0, 0, 128*8, 10))
	for c := 0; c < 128; c++ {
		for y := 0; y < 10; y++ {
			off := 0x550 + c*10 + y
			if off >= len(dat) {
				continue
			}
			for x := 0; x < 8; x++ {
				if dat[off]&(0x80>>uint(x)) != 0 {
					font.Set(c*8+x, y, color.White)
				}
			}
		}
	}
	if e := save(bundle.Font, font); e != nil {
		return e
	}
	// Players are two ordinary 16-pixel hardware sprites. Their two bitplanes
	// are interleaved by scanline; fourteen 120-byte strips form seven ship frames.
	for player := 1; player <= 2; player++ {
		for frame := 0; frame < 7; frame++ {
			start := frame * 240
			img := image.NewNRGBA(image.Rect(0, 0, 32, 30))
			for y := 0; y < 30; y++ {
				for x := 0; x < 32; x++ {
					off := start + x/16*120 + y*4
					bit := uint(15 - x%16)
					a := binary.BigEndian.Uint16(dat[off:])
					b := binary.BigEndian.Uint16(dat[off+2:])
					v := int((a>>bit)&1) | int((b>>bit)&1)<<1
					if v != 0 {
						colors := []uint16{0, 0xfdd, 0x889, 0x225}
						if player == 2 {
							colors = []uint16{0, 0xefd, 0xc94, 0x521}
						}
						c := colors[v]
						img.Set(x, y, color.RGBA{uint8(c>>8&15) * 17, uint8(c>>4&15) * 17, uint8(c&15) * 17, 255})
					}
				}
			}
			file := fmt.Sprintf("graphics/player_%d_%d.png", player, frame)
			if e := save(file, img); e != nil {
				return e
			}
			bundle.Sprites = append(bundle.Sprites, Sprite{ID: fmt.Sprintf("player_%d_%d", player, frame), File: file, Kind: "player", Width: 32, Height: 30, Address: uint32(0x10000 + start), Frame: frame})
		}
	}
	// Player explosions use two 60-line hardware strips and the palette bank
	// selected by the original death animation, rather than enemy BOB art.
	explosionPalette := bytes.Index(loader, []byte{0x0f, 0xff, 0x0f, 0xec, 0x0f, 0x91, 0x0f, 0xfd})
	if explosionPalette >= 0 {
		for frame := 0; frame < 10; frame++ {
			start := 0x3190 + frame*480
			img := image.NewNRGBA(image.Rect(0, 0, 32, 60))
			for y := 0; y < 60; y++ {
				for x := 0; x < 32; x++ {
					off := start + x/16*240 + y*4
					shift := uint(15 - x%16)
					v := int(binary.BigEndian.Uint16(dat[off:])>>shift&1) | int(binary.BigEndian.Uint16(dat[off+2:])>>shift&1)<<1
					if v == 0 {
						continue
					}
					rgb := binary.BigEndian.Uint16(loader[explosionPalette+frame*6+(v-1)*2:])
					img.Set(x, y, color.RGBA{uint8(rgb>>8&15) * 17, uint8(rgb>>4&15) * 17, uint8(rgb&15) * 17, 255})
				}
			}
			id := fmt.Sprintf("player_explosion_%d", frame)
			file := "graphics/" + id + ".png"
			if err := save(file, img); err != nil {
				return err
			}
			bundle.Sprites = append(bundle.Sprites, Sprite{ID: id, File: file, Kind: "player_explosion", Width: 32, Height: 60, Frame: frame, Address: uint32(0x10000 + start)})
		}
	}
	intro, e := read("lodint")
	if e != nil {
		return e
	}
	// The menu selects its own 32-colour preset before displaying LODINT.
	for i := 0; i < 32; i++ {
		v := binary.BigEndian.Uint16(loader[0x169e+i*2:])
		pal[i] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
	}
	if e := save(bundle.Title, planar(intro, 0, 320, 200, 5, 8000, 40, false)); e != nil {
		return e
	}
	if e := appendHUD(&bundle, loader, save); e != nil {
		return e
	}
	// These original scene overlays include the post-battle screen and story
	// artwork. Their display Copper lists specify five 8,000/8,320-byte planes.
	for _, scene := range []struct {
		name                          string
		width, height, planes, stride int
	}{{"lodtem", 640, 200, 4, 16000}, {"lodlod", 320, 208, 5, 8320}} {
		data, e := read(scene.name)
		if e != nil {
			return e
		}
		for i := 0; i < 32; i++ {
			preset := 0x179e
			if scene.name == "lodtem" {
				preset = 0x15fe
			}
			v := binary.BigEndian.Uint16(loader[preset+i*2:])
			pal[i] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
		}
		id := "scene_" + scene.name
		file := "graphics/" + id + ".png"
		if e := save(file, planar(data, 0, scene.width, scene.height, scene.planes, scene.stride, scene.width/8, false)); e != nil {
			return e
		}
		bundle.Sprites = append(bundle.Sprites, Sprite{ID: id, File: file, Kind: "scene", Width: scene.width, Height: scene.height})
	}
	final, e := read("lodfin")
	if e != nil {
		return e
	}
	if e := appendFinalBoss(&bundle, loader, final, save); e != nil {
		return e
	}
	for id := 0; id < 4; id++ {
		for i := 0; i < 32; i++ {
			off := []int{0x139a, 0x1426, 0x14b2, 0x153e}[id] + i*2
			v := binary.BigEndian.Uint16(loader[off:])
			pal[i] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
		}
		mem := make([]byte, 524288)
		copy(mem[0x100:], loader)
		copy(mem[0x10000:], dat)
		s0f, e := read("lods0f")
		if e != nil {
			return e
		}
		copy(mem[0x2e508:], s0f)
		s0s, e := read("lods0s")
		if e != nil {
			return e
		}
		copy(mem[0x3d800:], s0s)
		if id == 0 {
			tiles, e := read("lods0t")
			if e != nil {
				return e
			}
			copy(mem[0x44000:], tiles)
			if e := appendFinalBackdrop(&bundle, loader, mem, save); e != nil {
				return e
			}
		} else {
			name := fmt.Sprintf("lodst%d", id)
			stage, e := read(name)
			if e != nil {
				return e
			}
			base := []int{0, 0x2e89a, 0x2e4c0, 0x2e840}[id]
			copy(mem[base:], stage)
			// This SPIK revision already contains the runtime order. Its patched
			// loader returns before the older BOND loader's byte-reversal pass.
		}
		if e := appendCaveBoss(&bundle, mem, id, pal, save); e != nil {
			return e
		}
		if e := appendCatalog(&bundle, loader, mem, id, pal, save); e != nil {
			return e
		}
		tiles := make([]uint16, 512*24)
		img := image.NewNRGBA(image.Rect(0, 0, 384, 8192))
		atlas := image.NewNRGBA(image.Rect(0, 0, 256, 512))
		for row := 0; row < 512; row++ {
			for col := 0; col < 24; col++ {
				v := binary.BigEndian.Uint16(mem[0x44000+(row*24+col)*2:])
				tiles[row*24+col] = v
				for y := 0; y < 16; y++ {
					for x := 0; x < 16; x++ {
						index := 0
						for plane := 0; plane < 5; plane++ {
							off := 0x4a000 + int(v)*2 + plane*32 + y*2 + x/8
							if off < len(mem) {
								index |= int((mem[off]>>uint(7-x%8))&1) << plane
							}
						}
						img.Set(col*16+x, row*16+y, pal[index])
					}
				}
			}
		}
		for tile := 0; tile < 512; tile++ {
			single := planar(mem, 0x4a000+tile*160, 16, 16, 5, 32, 2, false)
			for y := 0; y < 16; y++ {
				for x := 0; x < 16; x++ {
					atlas.Set(tile%16*16+x, tile/16*16+y, single.At(x, y))
				}
			}
		}
		file := fmt.Sprintf("graphics/stage_%d.png", id)
		tf := fmt.Sprintf("graphics/tiles_%d.png", id)
		if e := save(file, img); e != nil {
			return e
		}
		if e := save(tf, atlas); e != nil {
			return e
		}
		bundle.Stages = append(bundle.Stages, Stage{ID: id, Mode: id, File: file, TilesFile: tf, Width: 384, Height: 8192, Rows: 512, TileBankAddress: 0x4a000, MapAddress: 0x44000, Tiles: tiles})
	}
	data, e := json.MarshalIndent(bundle, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, "manifest.json"), append(data, '\n'), 0644)
}
