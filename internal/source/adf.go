// Package source extracts the original Amiga disk without bundling copyrighted data.
package source

import (
	"encoding/binary"
	"fmt"
	"path"
	"strings"
)

const blockSize = 512

// File records one file in an Amiga OFS disk image.
type File struct {
	Path   string   `json:"path"`
	Header uint32   `json:"header_block"`
	Blocks []uint32 `json:"data_blocks"`
	Data   []byte   `json:"-"`
}

// ReadADF reads files from an Amiga original file system disk image.
func ReadADF(disk []byte) ([]File, error) {
	if len(disk) != 901120 || string(disk[:4]) != "DOS\x00" {
		return nil, fmt.Errorf("expected an 880 KiB Amiga OFS disk")
	}
	block := func(n uint32) ([]byte, error) {
		off := uint64(n) * blockSize
		if off+blockSize > uint64(len(disk)) {
			return nil, fmt.Errorf("block %d outside disk", n)
		}
		b := disk[off : off+blockSize]
		var sum uint32
		for i := 0; i < blockSize; i += 4 {
			sum += binary.BigEndian.Uint32(b[i:])
		}
		if sum != 0 {
			return nil, fmt.Errorf("OFS checksum mismatch at block %d", n)
		}
		return b, nil
	}
	word := func(b []byte, off int) uint32 { return binary.BigEndian.Uint32(b[off : off+4]) }
	root := word(disk, 8)
	if root == 0 {
		root = uint32(len(disk) / blockSize / 2)
	}
	seen := map[uint32]bool{}
	var files []File
	var visit func(uint32, string) error
	visit = func(n uint32, parent string) error {
		if seen[n] {
			return fmt.Errorf("filesystem cycle at block %d", n)
		}
		seen[n] = true
		b, err := block(n)
		if err != nil {
			return err
		}
		if word(b, 0) != 2 {
			return fmt.Errorf("block %d is not a file header", n)
		}
		kind := int32(word(b, 508))
		nameLen := int(b[432])
		if nameLen > 30 {
			return fmt.Errorf("invalid name length at block %d", n)
		}
		name := string(b[433 : 433+nameLen])
		if kind != 1 && (nameLen == 0 || name == "." || name == ".." || strings.ContainsAny(name, "/\\:\x00")) {
			return fmt.Errorf("unsafe OFS file name at block %d", n)
		}
		full := path.Join(parent, name)
		if kind == 1 {
			full = parent
		}
		switch kind {
		case 1, 2:
			for off := 24; off < 312; off += 4 {
				child := word(b, off)
				chain := map[uint32]bool{}
				for child != 0 {
					if chain[child] {
						return fmt.Errorf("hash chain cycle at %d", child)
					}
					chain[child] = true
					cb, err := block(child)
					if err != nil {
						return err
					}
					next := word(cb, 496)
					if err := visit(child, full); err != nil {
						return err
					}
					child = next
				}
			}
		case -3:
			size := word(b, 324)
			if size > uint32(len(disk)) {
				return fmt.Errorf("file %s exceeds disk size", full)
			}
			next := word(b, 16)
			data := make([]byte, 0, size)
			var blocks []uint32
			seq := uint32(1)
			chain := map[uint32]bool{}
			for next != 0 {
				if chain[next] {
					return fmt.Errorf("data cycle in %s", full)
				}
				chain[next] = true
				blocks = append(blocks, next)
				db, err := block(next)
				if err != nil {
					return err
				}
				if word(db, 0) != 8 || word(db, 4) != n || word(db, 8) != seq {
					return fmt.Errorf("invalid OFS data block %d for %s", next, full)
				}
				count := word(db, 12)
				if count > 488 {
					return fmt.Errorf("oversized OFS data block %d", next)
				}
				data = append(data, db[24:24+count]...)
				if uint32(len(data)) > size {
					return fmt.Errorf("OFS data exceeds declared size for %s", full)
				}
				next = word(db, 16)
				seq++
			}
			if uint32(len(data)) != size {
				return fmt.Errorf("size mismatch for %s: header %d, chain %d", full, size, len(data))
			}
			files = append(files, File{Path: full, Header: n, Blocks: blocks, Data: data})
		default:
			return fmt.Errorf("unsupported header kind %d at block %d", kind, n)
		}
		return nil
	}
	if err := visit(root, ""); err != nil {
		return nil, err
	}
	return files, nil
}
