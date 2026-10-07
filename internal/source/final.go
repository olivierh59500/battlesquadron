package source

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// appendFinalBoss decodes all 98,208 original LODFIN bytes. Its body, head,
// shared masks and two twelve-frame gun banks are contiguous; their boundaries
// agree exactly with the loader's demonstrated graphics pointers and strides.
func appendFinalBoss(bundle *Bundle, loader, data []byte, save func(string, image.Image) error) error {
	if len(data) != 98208 {
		return fmt.Errorf("unexpected original LODFIN size %d", len(data))
	}
	palette := make(color.Palette, 32)
	for i := 0; i < 32; i++ {
		v := binary.BigEndian.Uint16(loader[0x153e+i*2:])
		palette[i] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
	}
	type frame struct{ colors, mask uint32 }
	add := func(id string, w, h, stride int, frames []frame) error {
		img := image.NewNRGBA(image.Rect(0, 0, w*len(frames), h))
		for index, f := range frames {
			for y := 0; y < h; y++ {
				for x := 0; x < w; x++ {
					bit := uint(7 - x%8)
					offset := y*(w/8) + x/8
					mask := int(f.mask) - 0x44000 + offset
					if mask < 0 || mask >= len(data) {
						return fmt.Errorf("final mask outside LODFIN")
					}
					if data[mask]&(1<<bit) == 0 {
						continue
					}
					v := 0
					for plane := 0; plane < 5; plane++ {
						off := int(f.colors) - 0x44000 + plane*stride + offset
						if off < 0 || off >= len(data) {
							return fmt.Errorf("final colour plane outside LODFIN")
						}
						v |= int(data[off]>>bit&1) << plane
					}
					img.Set(index*w+x, y, palette[v])
				}
			}
		}
		file := "graphics/" + id + ".png"
		if err := save(file, img); err != nil {
			return err
		}
		bundle.Sprites = append(bundle.Sprites, Sprite{ID: id, File: file, Kind: "final_boss", Width: w, Height: h, Address: frames[0].colors, Stage: 4})
		return nil
	}
	body := []frame{{0x44000, 0x453b0}, {0x44690, 0x45500}, {0x44d20, 0x45650}}
	if err := add("final_body", 96, 28, 0x150, body); err != nil {
		return err
	}
	head := make([]frame, 15)
	for i := range head {
		mask := uint32(0x50060)
		if i >= 11 {
			mask += uint32(i-10) * 0x240
		}
		head[i] = frame{0x457a0 + uint32(i)*0xb40, mask}
	}
	if err := add("final_head", 96, 48, 0x240, head); err != nil {
		return err
	}
	for index, id := range []string{"final_gun_left", "final_gun_right"} {
		frames := make([]frame, 12)
		base := uint32(0x50ba0 + index*0x5a00)
		for i := range frames {
			colors := base + uint32(i)*0x780
			frames[i] = frame{colors, colors + 0x640}
		}
		if err := add(id, 80, 32, 0x140, frames); err != nil {
			return err
		}
	}
	return nil
}

// appendFinalBackdrop preserves the final 240 surface pixels already copied to
// the original terrain ring when LODFIN loads. The original branch selects its
// brown palette at Amiga $163E (loader file $153E) and then stops the scroll.
func appendFinalBackdrop(bundle *Bundle, loader, mem []byte, save func(string, image.Image) error) error {
	const width, height, startY = 384, 240, 8192 - 240
	palette := make(color.Palette, 32)
	for i := 0; i < 32; i++ {
		v := binary.BigEndian.Uint16(loader[0x153e+i*2:])
		palette[i] = color.RGBA{uint8(v>>8&15) * 17, uint8(v>>4&15) * 17, uint8(v&15) * 17, 255}
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		sourceY := startY + y
		mapRow, tileY := sourceY/16, sourceY%16
		for x := 0; x < width; x++ {
			tileX, bitX := x/16, x%16
			word := binary.BigEndian.Uint16(mem[0x44000+(mapRow*24+tileX)*2:])
			value := 0
			for plane := 0; plane < 5; plane++ {
				off := 0x4a000 + int(word)*2 + plane*32 + tileY*2 + bitX/8
				if off >= len(mem) {
					return fmt.Errorf("final backdrop tile outside original RAM")
				}
				value |= int(mem[off]>>uint(7-bitX%8)&1) << plane
			}
			img.Set(x, y, palette[value])
		}
	}
	const file = "graphics/final_backdrop.png"
	if err := save(file, img); err != nil {
		return err
	}
	bundle.Sprites = append(bundle.Sprites, Sprite{ID: "final_backdrop", File: file, Kind: "background", Width: width, Height: height, Address: 0x44000 + (startY/16)*48, Stage: 0})
	return nil
}
