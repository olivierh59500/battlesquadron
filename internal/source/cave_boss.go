package source

import (
	"fmt"
	"image"
	"image/color"
)

// appendCaveBoss reconstructs the multipart kind-nine graphics using the
// demonstrated body/turret headers, shared masks and palette of each cave.
// The resulting atlases keep the native controller's independent frame IDs.
func appendCaveBoss(bundle *Bundle, mem []byte, mode int, palette color.Palette, save func(string, image.Image) error) error {
	if mode < 1 || mode > 3 {
		return nil
	}
	// The genuine third-cave schedule contains no kind-nine multipart actor.
	// Its module begins above this ABI's shared body pointer, so those bytes
	// are not a demonstrated third-cave graphic and must not be padded/exported.
	if mode == 3 {
		return nil
	}
	type frame struct{ colors, mask int }
	add := func(role string, w, h, stride int, frames []frame) error {
		img := image.NewNRGBA(image.Rect(0, 0, w*len(frames), h))
		for index, f := range frames {
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					offset := y*(w/8) + x/8
					bit := uint(7 - x%8)
					mask := f.mask + offset
					if mask < 0 || mask >= len(mem) {
						return fmt.Errorf("cave boss mask outside original RAM")
					}
					if mem[mask]&(1<<bit) == 0 {
						continue
					}
					v := 0
					for plane := 0; plane < 5; plane++ {
						off := f.colors + plane*stride + offset
						if off < 0 || off >= len(mem) {
							return fmt.Errorf("cave boss colour plane outside original RAM")
						}
						v |= int(mem[off]>>bit&1) << plane
					}
					img.Set(index*w+x, y, palette[v])
				}
			}
		}
		id := fmt.Sprintf("boss_%d_%s", mode, role)
		file := "graphics/" + id + ".png"
		if err := save(file, img); err != nil {
			return err
		}
		bundle.Sprites = append(bundle.Sprites, Sprite{ID: id, File: file, Kind: "cave_boss", Width: w, Height: h, Address: uint32(frames[0].colors), Stage: mode})
		return nil
	}
	if mode == 1 {
		body := make([]frame, 4)
		for i := range body {
			colors := 0x2f560 + i*0x900
			body[i] = frame{colors, colors + 0x780}
		}
		if err := add("body", 96, 32, 0x180, body); err != nil {
			return err
		}
		turret := make([]frame, 4)
		for i := range turret {
			colors := 0x31960 + i*0xd80
			turret[i] = frame{colors, colors + 0xb40}
		}
		return add("turret", 96, 48, 0x240, turret)
	}
	// Modes two and three share the body ABI: six regular frames, one flash
	// and one damaged state, followed by a single 768-byte cookie-cut mask.
	body := make([]frame, 8)
	for i := range body {
		body[i] = frame{0x2e4c0 + i*0xf00, 0x35cc0}
	}
	if err := add("body", 128, 48, 0x300, body); err != nil {
		return err
	}
	// The two regular turret states select the same pixels; the Copper/clock
	// phase still distinguishes their state IDs before a flash or damage.
	turret := []frame{{0x35fc0, 0x37208}, {0x35fc0, 0x37208}, {0x365d8, 0x37208}, {0x36bf0, 0x37208}}
	return add("turret", 64, 39, 0x138, turret)
}
