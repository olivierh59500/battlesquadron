package source

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestSpikeInterleavedLiteralAndOverlappingMatch(t *testing.T) {
	// One literal A, then a two-byte copy one byte behind the cursor. Control
	// bits share a stream with literal bytes, so the next bit byte follows A.
	input := make([]byte, 47)
	copy(input, "SPIK")
	binary.BigEndian.PutUint32(input[4:], 3)
	binary.BigEndian.PutUint32(input[8:], 3)
	for i := 12; i < 44; i++ {
		input[i] = byte(i)
	}
	copy(input[44:], []byte{0x80, 'A', 0})
	out, err := UnpackSPIK(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out[:3], []byte("AAA")) || !bytes.Equal(out[3:], input[12:44]) {
		t.Fatalf("literal/match/tail decoding changed: %x", out)
	}
}

func TestSpikeRejectsDamagedStreams(t *testing.T) {
	for _, input := range [][]byte{nil, []byte("SPIK"), make([]byte, 44)} {
		if _, err := UnpackSPIK(input); err == nil {
			t.Fatal("accepted a missing or incomplete header")
		}
	}
	input := make([]byte, 46)
	copy(input, "SPIK")
	binary.BigEndian.PutUint32(input[4:], 2)
	binary.BigEndian.PutUint32(input[8:], 2)
	if _, err := UnpackSPIK(input); err == nil {
		t.Fatal("accepted a backreference before the output")
	}
	input[44] = 0x80
	if _, err := UnpackSPIK(input); err == nil {
		t.Fatal("accepted a truncated match control byte")
	}
	binary.BigEndian.PutUint32(input[8:], 9*1024*1024)
	if _, err := UnpackSPIK(input); err == nil {
		t.Fatal("accepted an excessive allocation request")
	}
}

func originalDisk(t *testing.T) []byte {
	t.Helper()
	names, err := filepath.Glob("../../previous/*.adf")
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 1 {
		t.Skip("the original ADF is intentionally excluded from Git")
	}
	data, err := os.ReadFile(names[0])
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateDisk(data); err != nil {
		t.Fatal(err)
	}
	return data
}

func TestVerifiedDiskAndDecodedChecksums(t *testing.T) {
	disk := originalDisk(t)
	files, err := ReadADF(disk)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 27 {
		t.Fatalf("recovered %d files, expected27", len(files))
	}
	expected := map[string]string{"loddat": "9915831dd37641d556fd4b2c081a1d085d0664a27a95ad7da99ffa973e80e37b", "lods0t": "7803866fc80d95e4768b05548556f9460a4bea80f792a60d439f1a3f80250171", "BattleDOS": "8d47738b722e304d7a07810d0bab6785fb3446bab4ca604643c3945221139f10"}
	for _, file := range files {
		want, ok := expected[file.Path]
		if !ok {
			continue
		}
		var data []byte
		if file.Path == "BattleDOS" {
			data, err = LoaderFromBattleDOS(file.Data)
		} else {
			data, err = UnpackSPIK(file.Data)
		}
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if hex.EncodeToString(sum[:]) != want {
			t.Errorf("decoded %s checksum changed", file.Path)
		}
		delete(expected, file.Path)
	}
	if len(expected) != 0 {
		t.Errorf("missing originals: %v", expected)
	}
}

func TestOFSRejectsChecksumCycleAndUnsafeName(t *testing.T) {
	original := originalDisk(t)
	checksum := func(block []byte) {
		binary.BigEndian.PutUint32(block[20:], 0)
		var sum uint32
		for off := 0; off < 512; off += 4 {
			sum += binary.BigEndian.Uint32(block[off:])
		}
		binary.BigEndian.PutUint32(block[20:], -sum)
	}
	for _, kind := range []string{"checksum", "cycle", "name", "size", "bounds"} {
		t.Run(kind, func(t *testing.T) {
			disk := bytes.Clone(original)
			block := disk[881*512 : 882*512]
			switch kind {
			case "checksum":
				block[100] ^= 1
			case "cycle":
				binary.BigEndian.PutUint32(block[496:], 881)
				checksum(block)
			case "name":
				block[432] = 2
				copy(block[433:], "..")
				checksum(block)
			case "size":
				binary.BigEndian.PutUint32(block[324:], 0xffffffff)
				checksum(block)
			case "bounds":
				binary.BigEndian.PutUint32(block[16:], 0xffffffff)
				checksum(block)
			}
			if _, err := ReadADF(disk); err == nil {
				t.Errorf("accepted damaged OFS %s", kind)
			}
		})
	}
}

func TestKnownSourceHashRejectsOtherRevision(t *testing.T) {
	if err := ValidateDisk(make([]byte, 901120)); err == nil {
		t.Fatal("accepted an unverified disk revision")
	}
}

func TestAssetsMatchOptionalOriginalEmulatorState(t *testing.T) {
	ram, err := os.ReadFile("../../.cache/reference/state2/CRAM.bin")
	if err != nil {
		t.Skip("optional FS-UAE verification capture is local and excluded from Git")
	}
	files, err := ReadADF(originalDisk(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Path != "loddat" && file.Path != "lods0t" {
			continue
		}
		data, err := UnpackSPIK(file.Data)
		if err != nil {
			t.Fatal(err)
		}
		base := 0x10000
		end := len(data)
		if file.Path == "lods0t" {
			base = 0x44000
		} else {
			end = 0x246f0 - base
		}
		for i := 0; i < end; i++ {
			address := base + i
			if file.Path == "loddat" && address >= 0x1737a && address < 0x17392 {
				continue
			}
			if data[i] != ram[address] {
				t.Fatalf("%s differs from original runtime at $%x: %02x/%02x", file.Path, address, data[i], ram[address])
			}
		}
	}
}
