package source

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// LoaderFromBattleDOS removes the cracker's executable wrapper and returns the
// original game loader at its fixed Amiga address of $100.
func LoaderFromBattleDOS(executable []byte) ([]byte, error) {
	offset := bytes.Index(executable, []byte("SPIK"))
	for offset >= 0 {
		if offset+44 <= len(executable) {
			size := int(binary.BigEndian.Uint32(executable[offset+4 : offset+8]))
			if size > 0 && offset+44+size <= len(executable) {
				data, e := UnpackSPIK(executable[offset : offset+44+size])
				if e == nil && len(data) > 0x9000 && bytes.Equal(data[:4], []byte{0x61, 0, 0, 0x4e}) {
					return data, nil
				}
			}
		}
		next := bytes.Index(executable[offset+4:], []byte("SPIK"))
		if next < 0 {
			break
		}
		offset += 4 + next
	}
	return nil, fmt.Errorf("original SPIK game loader not found")
}
