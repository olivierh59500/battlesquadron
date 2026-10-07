package source

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"os"
	"testing"
)

// These fixtures contain checksums only. The original bitmap bytes stay local.
func TestNovaGraphicsAreRecoveredFromTheSeparateOriginalShotBank(t *testing.T) {
	files, err := ReadADF(originalDisk(t))
	if err != nil {
		t.Fatal(err)
	}
	var loader, resident []byte
	for _, file := range files {
		switch file.Path {
		case "BattleDOS":
			loader, err = LoaderFromBattleDOS(file.Data)
		case "loddat":
			resident, err = UnpackSPIK(file.Data)
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	var reference []struct {
		Graphic               int
		Address               uint32
		Height                int
		SourceHash, PixelHash string
	}
	data, err := os.ReadFile("nova_graphics_test.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &reference); err != nil {
		t.Fatal(err)
	}
	if len(reference) != 4 {
		t.Fatal("the four original Nova frames must be covered")
	}
	mem := make([]byte, 524288)
	copy(mem[0x100:], loader)
	copy(mem[0x10000:], resident)
	pixels := make(map[string]string)
	palette := make(color.Palette, 32)
	for index := range palette {
		palette[index] = color.Transparent
	}
	var bundle Bundle
	err = appendCatalog(&bundle, loader, mem, 0, palette, func(name string, img image.Image) error {
		if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 {
			return nil
		}
		bytes := make([]byte, 0, 16*16*4)
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				bytes = append(bytes, byte(r>>8), byte(g>>8), byte(b>>8), byte(a>>8))
			}
		}
		pixels[name] = fmt.Sprintf("%x", sha256.Sum256(bytes))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range reference {
		source := loader[expected.Address-0x100 : expected.Address-0x100+64]
		if got := fmt.Sprintf("%x", sha256.Sum256(source)); got != expected.SourceHash {
			t.Fatalf("Nova%d original interleaved words changed", expected.Graphic)
		}
		id := fmt.Sprintf("bullet_%d_0", expected.Graphic)
		file := "graphics/" + id + ".png"
		if got := pixels[file]; got != expected.PixelHash {
			t.Fatalf("Nova%d bitmap missing or changed: %s", expected.Graphic, got)
		}
		found := false
		for _, sprite := range bundle.Sprites {
			if sprite.ID != id {
				continue
			}
			found = true
			if sprite.Address != expected.Address || sprite.Width != 16 || sprite.Height != expected.Height || sprite.Graphic != expected.Graphic || sprite.Kind != "bullet" {
				t.Fatalf("Nova%d descriptor changed: %+v", expected.Graphic, sprite)
			}
		}
		if !found {
			t.Fatalf("Nova%d was omitted from the manifest", expected.Graphic)
		}
	}
}
