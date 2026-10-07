package game

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/png"
	"io/fs"
	"strconv"
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
	frames        []*ebiten.Image
	width, height int
}

// A paged texture keeps every original terrain row within mobile GPU limits.
type terrain struct {
	pages         []*ebiten.Image
	width, height int
}

type artwork struct {
	manifest                            manifest
	title, font                         *ebiten.Image
	sprites                             map[string]sprite
	stages                              map[int]terrain
	graphics                            map[int]string
	loader                              []byte
	objects                             map[objectKey]string
	glyphs                              [256]*ebiten.Image
	bullets                             map[int]string
	flying                              map[flyingKey]string
	players                             [2][7]string
	explosions                          [10]string
	hudNova                             [2]string
	touchButtons, touchStick, touchKnob *ebiten.Image
	density                             int
}

type flyingKey struct{ stage, kind, frame int }

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
	a := &artwork{sprites: make(map[string]sprite), stages: make(map[int]terrain), graphics: make(map[int]string), objects: make(map[objectKey]string), bullets: make(map[int]string), flying: make(map[flyingKey]string), density: 1}
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
		columns := max(1, a.font.Bounds().Dx()/8)
		for index := range a.glyphs {
			rect := image.Rect(index%columns*8, index/columns*10, (index%columns+1)*8, (index/columns+1)*10)
			if rect.In(a.font.Bounds()) {
				a.glyphs[index] = a.font.SubImage(rect).(*ebiten.Image)
			}
		}
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
		s := sprite{image: ebiten.NewImageFromImage(img), width: width, height: height}
		for y := 0; y+height <= s.image.Bounds().Dy(); y += height {
			for x := 0; x+width <= s.image.Bounds().Dx(); x += width {
				s.frames = append(s.frames, s.image.SubImage(image.Rect(x, y, x+width, y+height)).(*ebiten.Image))
			}
		}
		a.sprites[spec.ID] = s
		var first, second, third int
		switch {
		case strings.HasPrefix(spec.ID, "bullet_"):
			if n, _ := fmt.Sscanf(spec.ID, "bullet_%d_0", &first); n == 1 {
				a.bullets[first] = spec.ID
			}
		case strings.HasPrefix(spec.ID, "flying_"):
			if n, _ := fmt.Sscanf(spec.ID, "flying_%d_%d_stage_%d", &first, &second, &third); n == 3 {
				a.flying[flyingKey{third, first, second}] = spec.ID
			}
		case strings.HasPrefix(spec.ID, "player_explosion_"):
			if n, _ := fmt.Sscanf(spec.ID, "player_explosion_%d", &first); n == 1 && first >= 0 && first < len(a.explosions) {
				a.explosions[first] = spec.ID
			}
		case strings.HasPrefix(spec.ID, "player_"):
			if n, _ := fmt.Sscanf(spec.ID, "player_%d_%d", &first, &second); n == 2 && first >= 1 && first <= 2 && second >= 0 && second < 7 {
				a.players[first-1][second] = spec.ID
			}
		case strings.HasPrefix(spec.ID, "hud_nova_"):
			if n, _ := fmt.Sscanf(spec.ID, "hud_nova_%d", &first); n == 1 && first >= 1 && first <= 2 {
				a.hudNova[first-1] = spec.ID
			}
		}
		if spec.Kind == "object" {
			a.graphics[spec.Graphic] = spec.ID
		}
		if spec.TemplateAddress != 0 {
			a.objects[objectKey{spec.Stage, spec.TemplateAddress, spec.Frame}] = spec.ID
		}
	}
	for graphic := 84; graphic <= 87; graphic++ {
		if a.bullets[graphic] == "" {
			return nil, fmt.Errorf("original Nova graphic %d is missing; recreate assets from the ADF", graphic)
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
			draw.Draw(page, page.Bounds(), img, rect.Min, draw.Src)
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
	return a.drawSpriteAt(dst, id, float64(x), float64(y), frame)
}

func (a *artwork) drawSpriteAt(dst *ebiten.Image, id string, x, y float64, frame int) bool {
	s, ok := a.sprites[id]
	if !ok {
		return false
	}
	if len(s.frames) == 0 {
		return false
	}
	frame = max(0, frame) % len(s.frames)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(a.density), float64(a.density))
	op.GeoM.Translate(x*float64(a.density), y*float64(a.density))
	dst.DrawImage(s.frames[frame], op)
	return true
}

// Boss controllers name a source frame; the assets store its contiguous atlas.
func (a *artwork) drawIndexedSprite(dst *ebiten.Image, id string, x, y, frame int) bool {
	return a.drawIndexedSpriteAt(dst, id, float64(x), float64(y), frame)
}

func (a *artwork) drawIndexedSpriteAt(dst *ebiten.Image, id string, x, y float64, frame int) bool {
	if _, ok := a.sprites[id]; ok {
		return a.drawSpriteAt(dst, id, x, y, frame)
	}
	if split := strings.LastIndex(id, "_"); split >= 0 {
		if index, err := strconv.Atoi(id[split+1:]); err == nil {
			return a.drawSpriteAt(dst, id[:split], x, y, index)
		}
	}
	return false
}

func (a *artwork) drawGrowingSprite(dst *ebiten.Image, id string, x, y, frame, height int) bool {
	return a.drawGrowingSpriteAt(dst, id, float64(x), float64(y), frame, height)
}

func (a *artwork) drawGrowingSpriteAt(dst *ebiten.Image, id string, x, y float64, frame, height int) bool {
	s, ok := a.sprites[id]
	if !ok {
		return false
	}
	height = max(0, min(height, s.height))
	if height == 0 {
		return true
	}
	columns := max(1, s.image.Bounds().Dx()/s.width)
	row := max(0, frame) / columns
	column := max(0, frame) % columns
	rect := image.Rect(column*s.width, row*s.height, (column+1)*s.width, row*s.height+height)
	if !rect.In(s.image.Bounds()) {
		return false
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(float64(a.density), float64(a.density))
	op.GeoM.Translate(x*float64(a.density), y*float64(a.density))
	dst.DrawImage(s.image.SubImage(rect).(*ebiten.Image), op)
	return true
}

func (a *artwork) drawTerrain(dst *ebiten.Image, stage, scroll, camera int) {
	a.drawTerrainAt(dst, stage, float64(scroll), float64(camera))
}

func (a *artwork) drawTerrainAt(dst *ebiten.Image, stage int, scroll, camera float64) {
	t, ok := a.stages[stage]
	if !ok {
		return
	}
	// The original map pointer identifies the newest top scanline. The rows
	// below it are the terrain already written into the scrolling ring.
	start := max(0, float64(t.height)-scroll)
	x := max(0, min(camera, float64(t.width-288)))
	for page := int(start) / 512; page < len(t.pages) && float64(page*512) < start+208; page++ {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(a.density), float64(a.density))
		// Destination clipping selects the same rows without allocating a subimage.
		op.GeoM.Translate(-x*float64(a.density), (float64(page*512)-start)*float64(a.density))
		dst.DrawImage(t.pages[page], op)
	}
}

// Text uses the bitmap recovered from the game instead of a substitute font.
func (a *artwork) text(dst *ebiten.Image, value string, x, y int, ink color.Color) {
	if a.font == nil {
		return
	}
	value = strings.ToUpper(value)
	width, height := 8, 10
	r, g, b, alpha := ink.RGBA()
	for _, char := range value {
		if char == '\n' {
			y += height + 2
			continue
		}
		index := int(char)
		if index < 0 || index >= len(a.glyphs) {
			x += width
			continue
		}
		if glyph := a.glyphs[index]; glyph != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(float64(a.density), float64(a.density))
			op.GeoM.Translate(float64(x*a.density), float64(y*a.density))
			op.ColorScale.Scale(float32(r)/65535, float32(g)/65535, float32(b)/65535, float32(alpha)/65535)
			dst.DrawImage(glyph, op)
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
