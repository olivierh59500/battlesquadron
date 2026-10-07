package game

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/png"
	"io/fs"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/assets"
)

// Manifest records only original data locations; it is generated from the ADF.
type manifest struct {
	Title, Font, Loader string
	Sprites             []spriteSpec
	Stages              []stageSpec
}

type spriteSpec struct {
	ID, File, Kind         string
	Width, Height, Graphic int
	Address                uint32
	Frame, Stage           int
	TemplateAddress        uint32
}

type stageSpec struct {
	ID, Mode, Width, Height, Rows int
	File, TilesFile               string
	MapAddress, TileBankAddress   uint32
	Tiles                         []uint16
}

type sprite struct {
	image         *ebiten.Image
	width, height int
}

// A paged texture keeps every original terrain row within mobile GPU limits.
type terrain struct {
	pages         []*ebiten.Image
	width, height int
}

type artwork struct {
	manifest    manifest
	title, font *ebiten.Image
	sprites     map[string]sprite
	stages      map[int]terrain
	graphics    map[int]string
	loader      []byte
	objects     map[objectKey]string
}

type objectKey struct {
	stage    int
	template uint32
	frame    int
}

func loadArtwork() (*artwork, error) {
	data, err := fs.ReadFile(assets.Files, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("extract the original ADF: %w", err)
	}
	a := &artwork{sprites: make(map[string]sprite), stages: make(map[int]terrain), graphics: make(map[int]string), objects: make(map[objectKey]string)}
	if err := json.Unmarshal(data, &a.manifest); err != nil {
		return nil, err
	}
	load := func(name string) (image.Image, error) {
		reader, err := assets.Files.Open(name)
		if err != nil {
			return nil, fmt.Errorf("original asset %s: %w", name, err)
		}
		defer reader.Close()
		img, _, err := image.Decode(reader)
		return img, err
	}
	if a.manifest.Title != "" {
		img, err := load(a.manifest.Title)
		if err != nil {
			return nil, err
		}
		a.title = ebiten.NewImageFromImage(img)
	}
	if a.manifest.Font != "" {
		img, err := load(a.manifest.Font)
		if err != nil {
			return nil, err
		}
		a.font = ebiten.NewImageFromImage(img)
	}
	for _, spec := range a.manifest.Sprites {
		img, err := load(spec.File)
		if err != nil {
			return nil, err
		}
		width, height := spec.Width, spec.Height
		if width <= 0 {
			width = img.Bounds().Dx()
		}
		if height <= 0 {
			height = img.Bounds().Dy()
		}
		a.sprites[spec.ID] = sprite{ebiten.NewImageFromImage(img), width, height}
		if spec.Kind == "object" {
			a.graphics[spec.Graphic] = spec.ID
		}
		if spec.TemplateAddress != 0 {
			a.objects[objectKey{spec.Stage, spec.TemplateAddress, spec.Frame}] = spec.ID
		}
	}
	for _, spec := range a.manifest.Stages {
		img, err := load(spec.File)
		if err != nil {
			return nil, err
		}
		t := terrain{width: img.Bounds().Dx(), height: img.Bounds().Dy()}
		for y := 0; y < t.height; y += 512 {
			end := min(y+512, t.height)
			rect := image.Rect(0, y, t.width, end)
			page := image.NewRGBA(image.Rect(0, 0, t.width, end-y))
			for py := rect.Min.Y; py < rect.Max.Y; py++ {
				for x := 0; x < t.width; x++ {
					page.Set(x, py-y, img.At(x, py))
				}
			}
			t.pages = append(t.pages, ebiten.NewImageFromImage(page))
		}
		a.stages[spec.ID] = t
	}
	loader := a.manifest.Loader
	if loader == "" {
		loader = "unpacked/loader.bin"
	}
	a.loader, err = fs.ReadFile(assets.Files, loader)
	if err != nil {
		return nil, err
	}
	return a, nil
}

func (a *artwork) drawSprite(dst *ebiten.Image, id string, x, y int, frame int) bool {
	s, ok := a.sprites[id]
	if !ok {
		return false
	}
	bounds := s.image.Bounds()
	columns := max(1, bounds.Dx()/s.width)
	rows := max(1, bounds.Dy()/s.height)
	frame = max(0, frame) % (columns * rows)
	rect := image.Rect((frame%columns)*s.width, (frame/columns)*s.height, (frame%columns+1)*s.width, (frame/columns+1)*s.height)
	if !rect.In(bounds) {
		return false
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(x), float64(y))
	dst.DrawImage(s.image.SubImage(rect).(*ebiten.Image), op)
	return true
}

func (a *artwork) drawTerrain(dst *ebiten.Image, stage, scroll, camera int) {
	t, ok := a.stages[stage]
	if !ok {
		return
	}
	// The original map pointer identifies the newest top scanline. The rows
	// below it are the terrain already written into the scrolling ring.
	start := max(0, t.height-scroll)
	for py := 0; py < 208; {
		y := start + py
		if y >= t.height {
			break
		}
		page := y / 512
		row := y % 512
		if page >= len(t.pages) {
			break
		}
		height := min(208-py, t.pages[page].Bounds().Dy()-row)
		x := max(0, min(camera, t.width-288))
		width := min(288, t.width-x)
		img := t.pages[page].SubImage(image.Rect(x, row, x+width, row+height)).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, float64(py))
		dst.DrawImage(img, op)
		py += height
	}
}

// Text uses the bitmap recovered from the game instead of a substitute font.
func (a *artwork) text(dst *ebiten.Image, value string, x, y int, ink color.Color) {
	if a.font == nil {
		return
	}
	value = strings.ToUpper(value)
	width, height := 8, 10
	columns := max(1, a.font.Bounds().Dx()/width)
	r, g, b, alpha := ink.RGBA()
	for _, char := range value {
		if char == '\n' {
			y += height + 2
			continue
		}
		index := int(char)
		if index < 0 {
			x += width
			continue
		}
		rect := image.Rect(index%columns*width, index/columns*height, (index%columns+1)*width, (index/columns+1)*height)
		if rect.In(a.font.Bounds()) {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(x), float64(y))
			op.ColorScale.Scale(float32(r)/65535, float32(g)/65535, float32(b)/65535, float32(alpha)/65535)
			dst.DrawImage(a.font.SubImage(rect).(*ebiten.Image), op)
		}
		x += width
	}
}

func originalWords(name string) ([]uint16, error) {
	if name == "" {
		return nil, nil
	}
	b, err := fs.ReadFile(assets.Files, name)
	if err != nil {
		return nil, err
	}
	if len(b)%2 != 0 {
		return nil, fmt.Errorf("odd original map length: %s", name)
	}
	out := make([]uint16, len(b)/2)
	for i := range out {
		out[i] = binary.BigEndian.Uint16(b[i*2:])
	}
	return out, nil
}
