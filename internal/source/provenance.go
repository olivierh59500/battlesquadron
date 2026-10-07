package source

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DiskSHA256 identifies the exact disk revision whose native data formats were verified.
const DiskSHA256 = "de335a312577f757ea699bd9a19162df9f5e89d83b7ca8a8deeac7ff0116609f"

// ValidateDisk rejects different disk revisions before any generated asset is overwritten.
func ValidateDisk(disk []byte) error {
	sum := sha256.Sum256(disk)
	actual := hex.EncodeToString(sum[:])
	if actual != DiskSHA256 {
		return fmt.Errorf("unsupported ADF SHA-256 %s; this conversion targets %s", actual, DiskSHA256)
	}
	return nil
}

type originalRecord struct {
	Path                      string
	HeaderBlock, HeaderOffset uint32
	DataBlocks, DataOffsets   []uint32
	Size                      int
	SHA256                    string
}
type derivedRecord struct {
	Path         string
	Size         int
	SHA256       string
	AmigaAddress uint32
}

// WriteProvenance records every recovered disk file and decoded resource with
// its original disk blocks and a cryptographic checksum. No source bytes enter Git.
func WriteProvenance(root string, files []File) error {
	provenance := struct {
		DiskSHA256 string
		Originals  []originalRecord
		Decoded    []derivedRecord
	}{DiskSHA256: DiskSHA256}
	digest := func(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }
	for _, f := range files {
		r := originalRecord{Path: f.Path, HeaderBlock: f.Header, HeaderOffset: f.Header * 512, DataBlocks: f.Blocks, Size: len(f.Data), SHA256: digest(f.Data)}
		for _, b := range f.Blocks {
			r.DataOffsets = append(r.DataOffsets, b*512+24)
		}
		provenance.Originals = append(provenance.Originals, r)
	}
	addresses := map[string]uint32{"loader": 0x100, "loddat": 0x10000, "lodsto": 0x62000, "lodint": 0x62000, "lodjoy": 0x78000, "lods0f": 0x2e508, "lods0s": 0x3d800, "lods0t": 0x44000, "lodst1": 0x2e89a, "lodst2": 0x2e4c0, "lodst3": 0x2e840, "lodgam": 0x246f0, "lodmus": 0x3d800, "lodcom": 0x3d800, "lodspe": 0x246f0, "lodhis": 0x246f0, "lodend": 0x62000, "lodfin": 0x44000, "lodtem": 0x62000, "lodlod": 0x30000, "lodtxt": 0x7f000, "lodsco": 0, "lodsav": 0x62000}
	entries, e := os.ReadDir(filepath.Join(root, "unpacked"))
	if e != nil {
		return e
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, e := os.ReadFile(filepath.Join(root, "unpacked", entry.Name()))
		if e != nil {
			return e
		}
		provenance.Decoded = append(provenance.Decoded, derivedRecord{Path: "unpacked/" + entry.Name(), Size: len(data), SHA256: digest(data), AmigaAddress: addresses[strings.TrimSuffix(entry.Name(), ".bin")]})
	}
	data, e := json.MarshalIndent(provenance, "", "  ")
	if e != nil {
		return e
	}
	return os.WriteFile(filepath.Join(root, "provenance.json"), append(data, '\n'), 0644)
}
