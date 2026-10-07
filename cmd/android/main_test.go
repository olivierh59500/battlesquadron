package main

import (
	"debug/elf"
	"testing"
)

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
