package source

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"image"
	"image/color"
	"slices"
)

// appendCatalog renders original object, masked flying-object and hardware
// projectile records. The source addresses come from the supplied game loader.
func appendCatalog(bundle *Bundle, loader, mem []byte, stage int, pal color.Palette, save func(string, image.Image) error) error {
	copper := bytes.Index(loader, []byte{1, 0x80, 0, 0, 1, 0x82, 6, 0x10, 1, 0x84, 0xd, 0xdd})
	if copper < 0 || copper+32*4 > len(loader) {
		return fmt.Errorf("original hardware sprite Copper palette not found")
	}
	hardwarePalette := color.Palette{color.Transparent}
	for index := 25; index <= 27; index++ {
		rgb := binary.BigEndian.Uint16(loader[copper+index*4+2:])
		hardwarePalette = append(hardwarePalette, color.NRGBA{uint8(rgb>>8&15) * 17, uint8(rgb>>4&15) * 17, uint8(rgb&15) * 17, 255})
	}
	add := func(spec Sprite, start, w, h, planes, stride, rowBytes, mask int, hardware bool) error {
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		visible := false
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				bit := uint(7 - x%8)
				v := 0
				if hardware {
					off := start + y*4
					shift := uint(15 - x)
					if off+4 > len(mem) {
						continue
					}
					v = int(binary.BigEndian.Uint16(mem[off:])>>shift&1) | int(binary.BigEndian.Uint16(mem[off+2:])>>shift&1)<<1
				} else {
					for p := 0; p < planes; p++ {
						off := start + p*stride + y*rowBytes + x/8
						if off < 0 || off >= len(mem) {
							continue
						}
						v |= int(mem[off]>>bit&1) << p
					}
				}
				if mask >= 0 {
					off := mask + y*rowBytes + x/8
					if off < 0 || off >= len(mem) || mem[off]&(1<<bit) == 0 {
						continue
					}
				} else if v == 0 {
					continue
				}
				visible = true
				if hardware {
					img.Set(x, y, hardwarePalette[v])
				} else {
					img.Set(x, y, pal[v])
				}
			}
		}
		if !visible {
			return nil
		}
		if spec.Kind != "bullet" {
			spec.ID = fmt.Sprintf("%s_stage_%d", spec.ID, stage)
		}
		spec.File = "graphics/" + spec.ID + ".png"
		spec.Width = w
		spec.Height = h
		spec.Stage = stage
		spec.Address = uint32(start)
		if err := save(spec.File, img); err != nil {
			return err
		}
		bundle.Sprites = append(bundle.Sprites, spec)
		return nil
	}
	rules := engine.DecodeMapRules(loader, 0x100)
	if stage == 0 {
		gates, err := engine.DecodeSurfaceGates(loader, 0x100)
		if err != nil {
			return err
		}
		rules = append(rules, engine.MapRule{Mode: 0, Definition: gates[0].Definition})
	}
	for _, rule := range rules {
		if rule.Mode != stage {
			continue
		}
		d := rule.Definition
		if d.GraphicAddress == 0 {
			continue
		}
		off := int(d.Address) - 0x100
		step := int(binary.BigEndian.Uint16(loader[off+26:]))
		if step == 0 {
			step = d.FrameStride
		}
		last := int(loader[off+33])
		if d.Kind == 0x27 {
			last = 1
		}
		if d.Kind < 6 {
			last = []int{15, 9, 11, 20, 17, 11}[d.Kind]
		}
		for frame := 0; frame <= max(d.Frame, last); frame++ {
			start := int(d.GraphicAddress) + step*frame
			spec := Sprite{ID: fmt.Sprintf("object_%d_%d_%d", d.Kind, frame, d.Address), Kind: "object", Graphic: int(d.Kind), Frame: frame, TemplateAddress: d.Address}
			if err := add(spec, start, d.Width, d.Height, 5, d.PlaneStride, d.Width/8, -1, false); err != nil {
				return err
			}
		}
	}
	// The original masked-object descriptors share the same five-plane layout.
	// Their source and mask pointers are separate because the OCS cookie-cut blit
	// applies one mask to every colour plane.
	descriptorBase := bytes.Index(loader, []byte{0, 0x20, 0, 3, 0, 0, 0, 0, 0, 0x20, 0, 0x20, 0, 0, 0, 0, 0, 1, 0x77, 0x80, 0, 1, 0x75, 0})
	if descriptorBase >= 0 {
		for kind := 0; kind < 15; kind++ {
			if kind == 9 && stage == 3 {
				continue
			}
			off := descriptorBase + kind*32
			if off+32 > len(loader) {
				break
			}
			d := loader[off:]
			h := int(binary.BigEndian.Uint16(d))
			words := int(binary.BigEndian.Uint16(d[2:]))
			w := (words - 1) * 16
			if w < 1 || w > 128 || h < 1 || h > 256 {
				continue
			}
			stride := (words - 1) * 2 * h
			gfx := int(binary.BigEndian.Uint32(d[20:]))
			mask := int(binary.BigEndian.Uint32(d[16:]))
			// Contiguous banks end at the next demonstrated source pointer.
			// The loader selects these direction and explosion frames explicitly.
			frames := []int{16, 10, 1, 4, 32, 12, 10, 3, 16, 1, 8, 1, 1, 4, 1}[kind]
			for frame := 0; frame < frames; frame++ {
				spec := Sprite{ID: fmt.Sprintf("flying_%d_%d", kind, frame), Kind: "flying", Graphic: kind, Frame: frame, TemplateAddress: uint32(descriptorBase + 0x100 + kind*32)}
				if err := add(spec, gfx+frame*stride*6, w, h, 5, stride, w/8, mask+frame*stride*6, false); err != nil {
					return err
				}
			}
		}
	}
	// Primary weapons use the ordinary two-bit hardware sprite lookup table.
	if stage == 0 {
		pointerBase := bytes.Index(loader, []byte{0, 1, 0, 0, 0, 1, 0, 0x78})
		weapons, _, err := engine.DecodeWeapons(loader, 0x100, 0)
		if err != nil {
			return err
		}
		heights := map[int]int{}
		// The two seven-line orb strips follow the primary-weapon entries.
		// They are referenced by the original projectile/copper graphic table.
		heights[0x58], heights[0x59] = 7, 7
		for _, weapon := range weapons {
			for _, level := range weapon {
				for _, shot := range level.Shots {
					heights[int(shot.Graphic)] = max(heights[int(shot.Graphic)], shot.Height)
				}
			}
		}
		// Nova templates live outside the twenty-four ordinary weapon banks.
		// Their four source graphics are also used by the eight emitted rays.
		// Omitting this separate bank left both visible Nova effects undecoded.
		nova, err := engine.DecodeNova(loader, 0x100, mem[0x10000:])
		if err != nil {
			return err
		}
		for _, shot := range nova.Shots {
			heights[int(shot.Graphic)] = max(heights[int(shot.Graphic)], shot.Height)
		}
		if pointerBase >= 0 {
			keys := make([]int, 0, len(heights))
			for key := range heights {
				keys = append(keys, key)
			}
			slices.Sort(keys)
			for _, graphic := range keys {
				h := heights[graphic]
				off := pointerBase + graphic*4
				if off+4 > len(loader) {
					continue
				}
				start := int(binary.BigEndian.Uint32(loader[off:]))
				if start == 0 {
					continue
				}
				spec := Sprite{ID: fmt.Sprintf("bullet_%d_0", graphic), Kind: "bullet", Graphic: graphic}
				if err := add(spec, start, 16, h, 2, 0, 4, -1, true); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
