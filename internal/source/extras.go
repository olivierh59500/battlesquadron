package source

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// appendHUD reconstructs the Nova counter artwork from the original Copper's
// sprite data words and per-line RGB4 writes. The same sprite is multiplexed
// across eight horizontal positions by SPR0POS; its pixels are preserved here.
func appendHUD(bundle *Bundle, loader []byte, save func(string, image.Image) error) error {
	start := bytes.Index(loader, []byte{0xe7, 0x21, 0xff, 0xfe, 0x01, 0x4e, 0, 0, 0x01, 0x4c, 0, 0})
	if start < 0 {
		return fmt.Errorf("Nova HUD Copper strip not found")
	}
	type row struct {
		a, b        uint16
		left, right [4]uint16
		drawn       bool
	}
	rows := make([]row, 13)
	line := 0xe7
	horizontal := 0
	var a, b uint16
	var palette [4]uint16
	for off := start; off+4 <= len(loader); off += 4 {
		reg := binary.BigEndian.Uint16(loader[off:])
		value := binary.BigEndian.Uint16(loader[off+2:])
		if reg&1 != 0 {
			line = int(reg >> 8)
			horizontal = int(reg & 0xff)
			if line >= 0xe7+13 {
				break
			}
			continue
		}
		y := line - 0xe7
		if y < 0 || y >= 13 {
			continue
		}
		switch reg {
		case 0x1a2:
			palette[1] = value
		case 0x1a4:
			palette[2] = value
		case 0x1a6:
			palette[3] = value
		case 0x146:
			b = value
		case 0x144:
			a = value
			if !rows[y].drawn {
				rows[y] = row{a: a, b: b, left: palette, right: palette, drawn: true}
			}
		}
		if horizontal >= 0x8f && rows[y].drawn {
			rows[y].right = palette
		}
	}
	for player := 1; player <= 2; player++ {
		img := image.NewNRGBA(image.Rect(0, 0, 16, 13))
		for y, r := range rows {
			palette := r.left
			if player == 2 {
				palette = r.right
			}
			for x := 0; x < 16; x++ {
				shift := uint(15 - x)
				v := int(r.a>>shift&1) | int(r.b>>shift&1)<<1
				if v == 0 {
					continue
				}
				rgb := palette[v]
				img.Set(x, y, color.RGBA{uint8(rgb>>8&15) * 17, uint8(rgb>>4&15) * 17, uint8(rgb&15) * 17, 255})
			}
		}
		id := fmt.Sprintf("hud_nova_%d", player)
		file := "graphics/" + id + ".png"
		if err := save(file, img); err != nil {
			return err
		}
		bundle.Sprites = append(bundle.Sprites, Sprite{ID: id, File: file, Kind: "hud", Width: 16, Height: 13, Address: uint32(start + 0x100)})
	}
	return nil
}
