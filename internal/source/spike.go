package source

import (
	"encoding/binary"
	"fmt"
)

// UnpackSPIK translates the disk's 68000 SPIKE bitstream decoder. The original
// decoder keeps the final 32 output bytes in the header to make in-place loading safe.
func UnpackSPIK(src []byte) ([]byte, error) {
	if len(src) < 44 || string(src[:4]) != "SPIK" {
		return nil, fmt.Errorf("missing SPIK header")
	}
	packed := int(binary.BigEndian.Uint32(src[4:8]))
	size := int(binary.BigEndian.Uint32(src[8:12]))
	if packed != len(src)-44 || size > 8*1024*1024 {
		return nil, fmt.Errorf("invalid SPIK sizes %d/%d", packed, size)
	}
	input := src[44:]
	pos := 0
	bits := byte(0)
	left := 0
	bit := func() (int, error) {
		if left == 0 {
			if pos >= len(input) {
				return 0, fmt.Errorf("truncated bitstream")
			}
			bits = input[pos]
			pos++
			left = 8
		}
		v := int(bits >> 7)
		bits <<= 1
		left--
		return v, nil
	}
	read := func(n int) (int, error) {
		v := 0
		for i := 0; i < n; i++ {
			b, e := bit()
			if e != nil {
				return 0, e
			}
			v = v*2 + b
		}
		return v, nil
	}
	out := make([]byte, 0, size+32)
	literal := false
	for len(out) < size {
		if !literal {
			b, e := bit()
			if e != nil {
				return nil, e
			}
			literal = b != 0
		}
		if literal {
			count := 1
			b, e := bit()
			if e != nil {
				return nil, e
			}
			if b != 0 {
				nb := []int{10, 3, 2, 1}
				base := []int{12, 5, 2, 1}
				for group := 3; group >= 0; group-- {
					v, e := read(nb[group])
					if e != nil {
						return nil, e
					}
					if v != (1<<nb[group])-1 || group == 0 {
						count = base[group] + v + 1
						break
					}
				}
			}
			if pos+count > len(input) {
				return nil, fmt.Errorf("truncated literals")
			}
			out = append(out, input[pos:pos+count]...)
			pos += count
			if len(out) >= size {
				break
			}
		}
		group := 4
		for group > 0 {
			b, e := bit()
			if e != nil {
				return nil, e
			}
			if b == 0 {
				break
			}
			group--
		}
		lens := []int{10, 2, 1, 0, 0}
		base := []int{9, 5, 3, 2, 1}
		v, e := read(lens[group])
		if e != nil {
			return nil, e
		}
		count := base[group] + v + 1
		slot, e := read(3)
		if e != nil {
			return nil, e
		}
		width := []int{4, 6, 6, 7, 8, 9, 10, 11}
		offsetBase := []int{1, 17, 81, 145, 273, 529, 1041, 2065}
		offset, e := read(width[slot])
		if e != nil {
			return nil, e
		}
		offset += offsetBase[slot]
		if offset > len(out) {
			return nil, fmt.Errorf("invalid backreference %d", offset)
		}
		for i := 0; i < count; i++ {
			out = append(out, out[len(out)-offset])
		}
		literal = false
	}
	if len(out) > size+1 {
		return nil, fmt.Errorf("SPIK output exceeds size: %d/%d", len(out), size)
	}
	out = out[:size]
	out = append(out, src[12:44]...)
	return out, nil
}
