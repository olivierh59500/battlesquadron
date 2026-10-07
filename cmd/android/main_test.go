package main

import (
	"debug/elf"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckDeviceSelection(t *testing.T) {
	tests := []struct {
		name  string
		check configuration
		valid bool
	}{
		{"emulator explicit", configuration{check: true, serial: "emulator-5580"}, true},
		{"emulator missing", configuration{check: true}, false},
		{"emulator rejects physical", configuration{check: true, serial: "67081JEA300033"}, false},
		{"physical explicit", configuration{physicalCheck: true, serial: "67081JEA300033"}, true},
		{"physical missing", configuration{physicalCheck: true}, false},
		{"physical rejects emulator", configuration{physicalCheck: true, serial: "emulator-5580"}, false},
		{"physical rejects path", configuration{physicalCheck: true, serial: "../device"}, false},
		{"mutually exclusive", configuration{check: true, physicalCheck: true, serial: "67081JEA300033"}, false},
		{"performance requires check", configuration{performance: true}, false},
		{"physical performance", configuration{performance: true, physicalCheck: true, serial: "67081JEA300033"}, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.check.validateChecks(); (err == nil) != test.valid {
				t.Fatalf("validateChecks() = %v; valid = %v", err, test.valid)
			}
		})
	}
}

func TestDeviceLeaseIsExclusiveAndPreservesForeignOwner(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	release, err := acquireDeviceLease("test-device")
	if err != nil {
		t.Fatal(err)
	}
	if other, err := acquireDeviceLease("test-device"); err == nil {
		other()
		t.Fatal("a second agent acquired an active device lease")
	}
	owner := filepath.Join(os.TempDir(), "codex-android-device-test-device.lock", "owner.txt")
	if err := os.WriteFile(owner, []byte("another agent owns this lease"), 0600); err != nil {
		t.Fatal(err)
	}
	release()
	if _, err := os.Stat(owner); err != nil {
		t.Fatalf("release removed another agent's owner record: %v", err)
	}
}

func TestNativePageAlignment(t *testing.T) {
	segment := func(kind elf.ProgType, alignment, address, size uint64) *elf.Prog {
		return &elf.Prog{ProgHeader: elf.ProgHeader{Type: kind, Align: alignment, Vaddr: address, Memsz: size}}
	}
	tests := []struct {
		name     string
		segments []*elf.Prog
		valid    bool
	}{
		{"aligned", []*elf.Prog{segment(elf.PT_LOAD, 16384, 0, 16384), segment(elf.PT_GNU_RELRO, 1, 32768, 16384)}, true},
		{"legacy load pages", []*elf.Prog{segment(elf.PT_LOAD, 4096, 0, 16384)}, false},
		{"partial relro page", []*elf.Prog{segment(elf.PT_LOAD, 16384, 0, 16384), segment(elf.PT_GNU_RELRO, 1, 32768, 4096)}, false},
		{"missing load segments", []*elf.Prog{segment(elf.PT_GNU_RELRO, 1, 32768, 16384)}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := verifySegments(test.segments)
			if (err == nil) != test.valid {
				t.Fatalf("verifySegments() = %v; valid = %v", err, test.valid)
			}
		})
	}
}
