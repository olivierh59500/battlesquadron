package engine

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// NovaConfig preserves the original expanding eight-ray table and shot bank.
type NovaConfig struct {
	Divisors []int16
	Vectors  [64][2]int16
	Shots    [12]Shot
}

// NovaRay is one original sprite effect around the ship firing its Nova.
type NovaRay struct {
	X, Y    int
	Graphic uint8
}

// DecodeNova reads the original LODDAT ray vectors and the loader's shot bank.
func DecodeNova(loader []byte, base uint32, dat []byte) (*NovaConfig, error) {
	if len(dat) < 0x737a {
		return nil, fmt.Errorf("original Nova vector bank is missing")
	}
	c := &NovaConfig{}
	for at := 0x71b0; at+2 <= 0x727a; at += 2 {
		v := int16(binary.BigEndian.Uint16(dat[at:]))
		if v < 0 {
			break
		}
		c.Divisors = append(c.Divisors, v)
	}
	if len(c.Divisors) == 0 {
		return nil, fmt.Errorf("original Nova divisor stream is empty")
	}
	for i := range c.Vectors {
		off := 0x727a + i*4
		c.Vectors[i] = [2]int16{int16(binary.BigEndian.Uint16(dat[off:])), int16(binary.BigEndian.Uint16(dat[off+2:]))}
	}
	// The original shot sounds directly follow the twelve Nova records.
	end := bytes.Index(loader, []byte{0x30, 0x36, 0x31, 0x37})
	if end < 144 {
		return nil, fmt.Errorf("original Nova projectile bank is missing")
	}
	for i := range c.Shots {
		r := loader[end-144+i*12:]
		c.Shots[i] = Shot{X: signedWord(r), Y: signedWord(r[2:]), VX: signedWord(r[4:]), VY: signedWord(r[6:]), Height: int(r[8]), Graphic: r[9], Delay: int(r[10]), Damage: int(int8(r[11]))}
		if c.Shots[i].Graphic < 84 || c.Shots[i].Graphic > 87 || c.Shots[i].Damage != -1 {
			return nil, fmt.Errorf("invalid original Nova projectile%d at$%x", i, base+uint32(end-144+i*12))
		}
	}
	return c, nil
}

func (e *Engine) updateNova() {
	e.NovaRays = e.NovaRays[:0]
	if e.NovaFrames == 0 || e.Data.Nova == nil {
		return
	}
	if e.novaIndex >= len(e.Data.Nova.Divisors) {
		e.NovaFrames = 0
		return
	}
	e.NovaFrames--
	p := e.Players[e.NovaOwner]
	divisor := int(e.Data.Nova.Divisors[e.novaIndex]) | 1
	e.novaIndex++
	for ray := 0; ray < 8; ray++ {
		v := e.Data.Nova.Vectors[(e.Frame&7)+ray*8]
		x, y := int(v[0])/divisor+p.X+8, -int(v[1])/divisor+p.Y-16
		e.NovaRays = append(e.NovaRays, NovaRay{x, y, uint8(84 + (e.Frame+ray)&3)})
	}
	if e.NovaFrames >= 176 && e.NovaFrames&15 == 14 {
		kept := e.PlayerShots[:0]
		for _, shot := range e.PlayerShots {
			if shot.Player != e.NovaOwner {
				kept = append(kept, shot)
			}
		}
		e.PlayerShots = kept
		for slot, t := range e.Data.Nova.Shots {
			e.PlayerShots = append(e.PlayerShots, Bullet{X: p.X + t.X, Y: p.Y + t.Y, VX: t.VX, VY: t.VY, Width: 48, Height: t.Height, Damage: -1, Delay: t.Delay, Graphic: t.Graphic, Player: e.NovaOwner, Slot: slot, originX: t.X, originY: t.Y, Nova: true})
		}
	}
}
