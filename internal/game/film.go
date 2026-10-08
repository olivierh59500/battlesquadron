package game

import (
	"encoding/hex"
	"fmt"
	"image/color"
	"io"
	"io/fs"
	"time"
	"unicode/utf8"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/assets"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/showcase"
	"github.com/olivierh59500/battlesquadron/internal/sound"
)

// Film renders an editorial presentation using the real menus and expert inputs.
// It owns its native PCM consumer and never captures a microphone or OS audio.
type Film struct {
	game      *Game
	logical   *ebiten.Image
	deaths    int
	audioSkip []byte
}

// FilmState records native state for the presentation's provenance report.
type FilmState struct {
	Frame, Stage, Progress, Mode, Weapon, Level, Nova, Lives, Score, Deaths int
	InputPosition                                                           uint64
	ActiveDemo                                                              bool
	NativeSHA256                                                            string
}

// NewFilm loads the same original resources as the application.
func NewFilm() (*Film, error) {
	g, err := New()
	if err != nil {
		return nil, err
	}
	read := func(name string) ([]byte, error) { return fs.ReadFile(assets.Files, "unpacked/"+name+".bin") }
	var banks sound.Assets
	for _, bank := range []struct {
		name string
		out  *[]byte
	}{{"lodgam", &banks.Game}, {"lodmus", &banks.Title}, {"lodcom", &banks.Common}, {"lodspe", &banks.Special}} {
		*bank.out, err = read(bank.name)
		if err != nil {
			return nil, err
		}
	}
	g.sound, err = sound.NewWithAssets(banks, 44100)
	if err != nil {
		return nil, err
	}
	g.sound.UseMenu(true)
	g.sound.PlayTrack(1)
	return &Film{game: g, logical: ebiten.NewImage(Width, Height), audioSkip: make([]byte, 882*4)}, nil
}

// Step advances exactly one presentation PAL field. Edited gameplay excerpts
// retain every omitted input and native resource change before the next cut.
func (f *Film) Step(chapter showcase.Chapter, tick int, stamp time.Time) error {
	if tick < 0 || tick >= chapter.DurationFields {
		return fmt.Errorf("invalid presentation tick %d for %s", tick, chapter.Name)
	}
	if tick == 0 {
		if err := f.enterChapter(chapter, stamp); err != nil {
			return err
		}
	}
	g := f.game
	switch chapter.Page {
	case showcase.PageTitle:
		// Opening title remains visible before the first scripted menu action.
	case showcase.PagePlayers:
		if tick == 0 || tick == 350 {
			f.menuPointer(90, 120)
		} else if tick == 100 {
			f.menuPointer(130, 120)
		}
	case showcase.PageOptions:
		switch tick {
		case 100:
			f.menuPointer(208, 94)
		case 200:
			f.menuPointer(208, 174)
		case 350:
			f.menuPointer(208, 134)
		case 425:
			f.menuPointer(208, 154)
		case 550:
			f.menuPointer(96, 94)
			f.menuPointer(96, 174)
			f.menuPointer(96, 134)
			f.menuPointer(96, 154)
		}
	case showcase.PageScores:
	case showcase.PageIdle:
		if _, err := g.updateAttract(playerActivity{}); err != nil {
			return err
		}
	case showcase.PageExpert:
		if !g.DemoActive() {
			return fmt.Errorf("expert excerpt started before the real idle demonstration")
		}
		if _, err := g.updateAttract(playerActivity{}); err != nil {
			return err
		}
		f.countEvents()
	case showcase.PageOutro:
		// The ending and its completed bonus remain untouched for the final card.
	default:
		return fmt.Errorf("unknown presentation page %q", chapter.Page)
	}
	g.observePresentationAt(stamp)
	return g.sound.Err()
}

func (f *Film) enterChapter(chapter showcase.Chapter, stamp time.Time) error {
	g := f.game
	if chapter.Page == showcase.PageExpert {
		if !g.DemoActive() {
			return fmt.Errorf("the fifteen-second idle menu did not start its demo")
		}
		position := g.attract.cursor.Position()
		if position > chapter.InputStart {
			return fmt.Errorf("expert presentation cuts cannot rewind native input")
		}
		skipped := chapter.InputStart - position
		if skipped != 0 {
			g.resetPresentation()
		}
		for field := uint64(0); field < skipped; field++ {
			if _, err := g.updateAttract(playerActivity{}); err != nil {
				return err
			}
			f.countEvents()
			g.observePresentationAt(stamp.Add(-time.Duration(skipped-field) * palField))
			if _, err := io.ReadFull(g.sound, f.audioSkip); err != nil {
				return err
			}
		}
		return nil
	}
	if chapter.Page == showcase.PageOutro {
		if g.Core.Mode != engine.Ending || !g.Core.Campaign.Completed() {
			return fmt.Errorf("the expert did not reach its complete native ending")
		}
		return nil
	}
	g.optionsOpen, g.showScores = false, false
	g.resetMenuIdle()
	switch chapter.Page {
	case showcase.PageOptions:
		f.menuPointer(90, 175)
	case showcase.PageScores:
		// This is the same native table selected by the title's H command.
		g.showScores = true
	}
	return nil
}

func (f *Film) menuPointer(x, y int) {
	f.game.resetMenuIdle()
	inputs := [2]engine.Input{}
	f.game.pointer(x, y, &inputs)
}

func (f *Film) countEvents() {
	for _, event := range f.game.Core.Events {
		if event.Kind == "death" {
			f.deaths++
		}
	}
}

// DrawFrame retains the production final viewport, including subpixel motion.
// Editorial captions use the original bitmap font outside the game picture.
func (f *Film) DrawFrame(dst *ebiten.Image, stamp time.Time, lines []string) {
	dst.Fill(color.Black)
	f.game.drawAt(f.logical, stamp)
	var geo ebiten.GeoM
	geo.Scale(3, 3)
	geo.Translate(160, 24)
	f.game.DrawFinalScreen(dst, f.logical, geo)
	art := f.game.art
	previous := art.density
	art.density = 3
	for index, line := range lines {
		x := (1280/3 - utf8.RuneCountInString(line)*8) / 2
		art.text(dst, line, x, 280+index*14, white)
	}
	art.density = previous
}

// ReadAudio advances only the original native sound sequencer and sample mixer.
func (f *Film) ReadAudio(out []byte) (int, error) { return f.game.sound.Read(out) }

// State returns a value snapshot without altering the recorded expert run.
func (f *Film) State() FilmState {
	g, p := f.game, f.game.Core.Players[0]
	digest := g.Core.Digest()
	return FilmState{Frame: g.Core.Frame, Stage: g.Core.Stage, Progress: g.Core.Scroll, Mode: int(g.Core.Mode),
		Weapon: p.Weapon, Level: p.Level, Nova: p.Nova, Lives: p.Lives, Score: p.Score,
		Deaths: f.deaths, InputPosition: g.attract.cursor.Position(), ActiveDemo: g.DemoActive(), NativeSHA256: hex.EncodeToString(digest[:])}
}
