// Command extract recreates unmodified disk resources from the supplied ADF.
package main

import (
	"flag"
	"fmt"
	"github.com/olivierh59500/battlesquadron/internal/source"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	input := flag.String("adf", "", "original OFS disk image")
	output := flag.String("out", "assets", "output directory")
	flag.Parse()
	if *input == "" {
		files, _ := filepath.Glob("previous/*.adf")
		if len(files) == 1 {
			*input = files[0]
		} else {
			fatal(fmt.Errorf("provide -adf"))
		}
	}
	disk, e := os.ReadFile(*input)
	fatal(e)
	fatal(source.ValidateDisk(disk))
	files, e := source.ReadADF(disk)
	fatal(e)
	for _, f := range files {
		target := filepath.Join(*output, "disk", filepath.FromSlash(f.Path))
		fatal(os.MkdirAll(filepath.Dir(target), 0755))
		fatal(os.WriteFile(target, f.Data, 0644))
		if f.Path == "BattleDOS" {
			loader, e := source.LoaderFromBattleDOS(f.Data)
			fatal(e)
			target = filepath.Join(*output, "unpacked", "loader.bin")
			fatal(os.MkdirAll(filepath.Dir(target), 0755))
			fatal(os.WriteFile(target, loader, 0644))
			fmt.Printf("loader %d bytes at Amiga address 0x100\n", len(loader))
		}
		if strings.HasPrefix(filepath.Base(f.Path), "lod") && len(f.Data) >= 4 && string(f.Data[:4]) == "SPIK" {
			data, e := source.UnpackSPIK(f.Data)
			if e != nil {
				fatal(fmt.Errorf("%s: %w", f.Path, e))
			}
			target = filepath.Join(*output, "unpacked", f.Path+".bin")
			fatal(os.MkdirAll(filepath.Dir(target), 0755))
			fatal(os.WriteFile(target, data, 0644))
			fmt.Printf("%-8s %6d -> %6d\n", f.Path, len(f.Data), len(data))
		}
	}
	fatal(source.WriteGraphics(*output))
	fatal(source.WriteProvenance(*output, files))
}
func fatal(e error) {
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
