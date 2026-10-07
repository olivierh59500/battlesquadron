package reference

import (
	"bytes"
	"compress/zlib"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Only checksum metadata is committed. Local reports remain optional ignored
// artifacts; when present, their bytes must retain the recorded provenance.
func TestOriginalReportChecksums(t *testing.T) {
	encoded, err := os.ReadFile("original_checksums_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var proofs []struct{ ReportPath, ReportHash string }
	if err := json.Unmarshal(encoded, &proofs); err != nil {
		t.Fatal(err)
	}
	for _, proof := range proofs {
		data, err := os.ReadFile(filepath.Join("../..", proof.ReportPath))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if actual := fmt.Sprintf("%x", sha256.Sum256(data)); actual != proof.ReportHash {
			t.Fatalf("local original report%s has different provenance", proof.ReportPath)
		}
	}
}

func recordedWords(words ...uint32) []byte {
	var data []byte
	for _, word := range words {
		data = binary.BigEndian.AppendUint32(data, word)
	}
	return data
}

func TestRecordingRetainsRasterAndSignedState(t *testing.T) {
	data := recordedWords(0x80000003, 0x10012345, 0x08023456, 0x4000007f, 0x2001003d, 0x80000004, 0x10012346, 0x08023457, 0x20ff003d)
	record, err := DecodeRecording(data)
	if err != nil {
		t.Fatal(err)
	}
	if record.LastFrame() != 4 || record.Frames[0].RandomChecksum != 0x12345 || record.Frames[0].StateChecksum != 0x23456 {
		t.Fatal("frame provenance was lost")
	}
	if event := record.Frames[0].Events[0]; event.Frame != 3 || event.Line != 127 || event.ID != 61 || event.State != 1 {
		t.Fatalf("raster input changed: %+v", event)
	}
	if event := record.Frames[1].Events[0]; event.Line != 0 || event.State != -1 {
		t.Fatalf("frame line or signed state changed: %+v", event)
	}
}

func TestRecordingRejectsTruncationAndInvalidMarkers(t *testing.T) {
	for _, data := range [][]byte{{0}, recordedWords(0x80000001), recordedWords(0x80000001, 0x10000000, 0), recordedWords(0x2001003d), recordedWords(0x80000001, 0x10000000, 0x08000000, 0x80000003, 0x10000000, 0x08000000)} {
		if _, err := DecodeRecording(data); err == nil {
			t.Fatal("invalid recording was accepted")
		}
	}
}

func TestHeldWindowRejectsARelease(t *testing.T) {
	record, err := DecodeRecording(recordedWords(0x80000001, 0x10000000, 0x08000000, 0x2001003d, 0x80000002, 0x10000000, 0x08000000, 0x2000003d))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := record.HeldRightWindow(1, 2, 61); err == nil {
		t.Fatal("release inside a constant-input window was accepted")
	}
}

func TestStateReadsCompressedRAMAndCorrectPCOffset(t *testing.T) {
	ram := make([]byte, 0x10000)
	put := func(address int, value uint16) { binary.BigEndian.PutUint16(ram[address:], value) }
	put(0x1058, 6094)
	put(0x9b86, 303)
	put(0x9b84, 305)
	for index := 0; index < 2; index++ {
		p := 0x4da2 + index*266
		put(p, 336)
		put(p+2, 374)
		put(p+10, 10)
		put(p+52, 74)
		put(p+58, 3)
		put(p+66, 3)
		ram[p+56] = 2
		copy(ram[p+106:p+114], "00000000")
	}
	var compressed bytes.Buffer
	writer := zlib.NewWriter(&compressed)
	if _, err := writer.Write(ram); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	packed := binary.BigEndian.AppendUint32(nil, uint32(len(ram)))
	packed = append(packed, compressed.Bytes()...)
	cycles, cpu := make([]byte, 16), make([]byte, 0x58)
	binary.BigEndian.PutUint32(cycles[4:], 512)
	binary.BigEndian.PutUint64(cycles[8:], 755706697216)
	binary.BigEndian.PutUint32(cpu[0x44:], 0xce0)
	binary.BigEndian.PutUint32(cpu[0x4c:], 0xc10c64)
	chunk := func(id string, flags uint32, payload []byte) []byte {
		size := len(payload) + 12
		result := append([]byte(id), recordedWords(uint32(size), flags)...)
		result = append(result, payload...)
		return append(result, make([]byte, 4-size%4)...)
	}
	data := append(chunk("CRAM", 1, packed), chunk("CYCS", 0, cycles)...)
	data = append(data, chunk("CPU ", 0, cpu)...)
	state, err := DecodeState(data)
	if err != nil {
		t.Fatal(err)
	}
	if state.PC != 0xce0 || state.CycleUnit != 512 || state.StoredCycles != 755706697216 || state.Clock != 6094 || state.Progress != 303 || state.Camera != 49 {
		t.Fatalf("state provenance changed: %+v", state)
	}
	if p := state.Players[0]; p.X != 80 || p.Y != 118 || p.Lives != 3 || p.Invulnerable != 74 || p.Weapon != 3 {
		t.Fatalf("original player scalars changed: %+v", p)
	}
}
