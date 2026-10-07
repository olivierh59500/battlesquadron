// Package game presents the native Go simulation through Ebitengine.
package game

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/battlesquadron/assets"
	"github.com/olivierh59500/battlesquadron/internal/controls"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/sound"
)

const Width, Height = 320, 256

// Game holds platform presentation; gameplay remains in the headless engine.
type Game struct {
	Core                                    *engine.Engine
	art                                     *artwork
	view                                    *ebiten.Image
	touch                                   *controls.Controller
	touchEnabled, paused, optionsOpen, mute bool
	musicEnabled, effectsEnabled            bool
	selectedOption, players                 int
	dataDir                                 string
	audioPlayer                             *audio.Player
	sound                                   *sound.Player
	updates, SmokeFrames                    int
	Capture                                 string
	captured                                bool
	touchIDs                                []ebiten.TouchID
	padIDs                                  []ebiten.GamepadID
	previousPointers                        map[int]controls.Point
	pointerSuppressed                       bool
	scores                                  []scoreEntry
	scorePlayers                            []int
	initials                                string
	initialIndex                            int
	showScores                              bool
	storageErr                              error
	lastFire                                [2]bool
}

// New requires every asset to have been reproduced from the original disk.
func New() (*Game, error) {
	a, err := loadArtwork()
	if err != nil {
		return nil, err
	}
	data := &engine.Data{Loader: a.loader, LoaderBase: 0x100}
	dat, err := fs.ReadFile(assets.Files, "unpacked/loddat.bin")
	if err != nil {
		return nil, err
	}
	if len(dat) >= 0x7500 {
		data.Random = append([]byte(nil), dat[0x7400:0x7500]...)
	}
	data.Nova, err = engine.DecodeNova(a.loader, 0x100, dat)
	if err != nil {
		return nil, err
	}
	overlays := make(map[string][]byte)
	for _, name := range []string{"lods0f", "lods0s", "lodst1", "lodst2", "lodst3"} {
		overlays[name], err = fs.ReadFile(assets.Files, "unpacked/"+name+".bin")
		if err != nil {
			return nil, err
		}
	}
	schedules, err := engine.NativeSchedules(a.loader, 0x100, overlays)
	if err != nil {
		return nil, err
	}
	for index, spec := range a.manifest.Stages {
		tiles := spec.Tiles
		data.Stages = append(data.Stages, engine.Stage{ID: spec.ID, Mode: spec.Mode, Width: 24, Height: spec.Height, Tiles: tiles, Events: schedules[spec.ID], Next: (index + 1) % len(a.manifest.Stages)})
	}
	core, err := engine.New(data)
	if err != nil {
		return nil, err
	}
	scores, err := originalScores()
	if err != nil {
		return nil, err
	}
	g := &Game{Core: core, art: a, view: ebiten.NewImage(288, 208), touch: controls.New(480, 256), players: 1, musicEnabled: true, effectsEnabled: true, scores: scores}
	return g, nil
}

// SetTouchEnabled enables Android's simultaneous stick, fire and Nova controls.
func (g *Game) SetTouchEnabled(enabled bool) { g.touchEnabled = enabled; g.CancelInput() }

// SetMuted suppresses audio initialization for command-line validation.
func (g *Game) SetMuted(mute bool) {
	g.mute = mute
	if g.audioPlayer != nil {
		if mute {
			g.audioPlayer.Pause()
		} else {
			g.audioPlayer.Play()
		}
	}
}

// AtTitle lets Android Back finish the native activity at the title screen.
func (g *Game) AtTitle() bool { return g.Core.Mode == engine.Title && !g.optionsOpen }

// Pause preserves a running session when the application loses focus.
func (g *Game) Pause() {
	if g.Core.Mode == engine.Playing {
		g.paused = true
	}
	g.CancelInput()
	if g.audioPlayer != nil {
		g.audioPlayer.Pause()
	}
}

// Back closes options, pauses gameplay, or returns a paused session to the title.
func (g *Game) Back() {
	if g.optionsOpen {
		g.optionsOpen = false
		g.saveSettings()
		return
	}
	if g.showScores {
		g.showScores = false
		return
	}
	if g.Core.Mode == engine.Playing && !g.paused {
		g.Pause()
		return
	}
	g.Core.ReturnToTitle()
	g.paused = false
	g.CancelInput()
	if g.sound != nil {
		g.sound.UseMenu(true)
	}
	if g.audioPlayer != nil && !g.mute {
		g.audioPlayer.Play()
	}
}

// CancelInput releases touch ownership and waits for held pointers to end.
func (g *Game) CancelInput() { g.touch.Cancel(); g.previousPointers = nil; g.pointerSuppressed = true }

func (g *Game) initializeAudio() error {
	if g.mute || g.sound != nil {
		return nil
	}
	read := func(name string) ([]byte, error) { return fs.ReadFile(assets.Files, "unpacked/"+name+".bin") }
	game, err := read("lodgam")
	if err != nil {
		return err
	}
	title, err := read("lodmus")
	if err != nil {
		return err
	}
	common, err := read("lodcom")
	if err != nil {
		return err
	}
	special, err := read("lodspe")
	if err != nil {
		return err
	}
	g.sound, err = sound.NewWithAssets(sound.Assets{Game: game, Title: title, Common: common, Special: special}, 44100)
	if err != nil {
		return err
	}
	g.sound.UseMenu(g.Core.Mode == engine.Title)
	g.sound.PlayTrack(1)
	g.sound.SetMusicEnabled(g.musicEnabled)
	g.sound.SetEffectsEnabled(g.effectsEnabled)
	context := audio.CurrentContext()
	if context == nil {
		context = audio.NewContext(44100)
	}
	g.audioPlayer, err = context.NewPlayer(g.sound)
	if err != nil {
		return err
	}
	g.audioPlayer.SetBufferSize(80_000_000)
	g.audioPlayer.Play()
	return nil
}

// Update receives one PAL tick from Ebitengine on desktop and Android.
func (g *Game) Update() error {
	if g.storageErr != nil {
		return fmt.Errorf("save settings: %w", g.storageErr)
	}
	if g.sound != nil && g.sound.Err() != nil {
		return g.sound.Err()
	}
	if g.SmokeFrames > 0 && g.updates >= g.SmokeFrames && (g.Capture == "" || g.captured) {
		return ebiten.Termination
	}
	if !ebiten.IsFocused() && g.SmokeFrames == 0 {
		g.Pause()
		return nil
	}
	if err := g.initializeAudio(); err != nil {
		return err
	}
	g.updates++
	if inpututil.IsKeyJustPressed(ebiten.KeyF11) {
		ebiten.SetFullscreen(!ebiten.IsFullscreen())
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Back()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyP) || inpututil.IsKeyJustPressed(ebiten.KeyPause) {
		g.togglePause()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF3) {
		g.effectsEnabled = !g.effectsEnabled
		if g.sound != nil {
			g.sound.SetEffectsEnabled(g.effectsEnabled)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF4) {
		g.musicEnabled = !g.musicEnabled
		if g.sound != nil {
			g.sound.SetMusicEnabled(g.musicEnabled)
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyM) {
		g.SetMuted(!g.mute)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyF5) && g.AtTitle() {
		g.optionsOpen = true
	}
	inputs := g.input()
	justFire := inputs[0].Fire && !g.lastFire[0] || inputs[1].Fire && !g.lastFire[1]
	g.lastFire = [2]bool{inputs[0].Fire, inputs[1].Fire}
	if inpututil.IsKeyJustPressed(ebiten.KeyH) && g.Core.Mode == engine.Title {
		g.showScores = !g.showScores
	}
	if len(g.scorePlayers) > 0 {
		g.enterInitials()
		return nil
	}
	if g.showScores {
		if justFire {
			g.showScores = false
		}
		return nil
	}
	if g.optionsOpen {
		g.updateOptions()
		return nil
	}
	if g.Core.Mode == engine.Title {
		if inpututil.IsKeyJustPressed(ebiten.KeyF1) {
			g.players = 1
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyF2) {
			g.players = 2
		}
		if justFire || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.start()
		}
		return nil
	}
	if g.Core.Mode == engine.GameOver || g.Core.Mode == engine.Ending {
		if justFire || inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
			g.Back()
		}
		return nil
	}
	if g.paused {
		return nil
	}
	if g.SmokeFrames > 0 {
		inputs[0].Fire = true
		inputs[0].X = 0
		if (g.updates/50)%2 == 0 {
			inputs[0].X = -1
		} else {
			inputs[0].X = 1
		}
	}
	g.Core.Tick(inputs)
	if g.Core.Mode == engine.GameOver || g.Core.Mode == engine.Ending {
		g.queueScores()
	}
	if g.sound != nil {
		for _, event := range g.Core.Events {
			switch event.Kind {
			case "shot":
				g.sound.PlayEffect([4]uint16{48, 54, 49, 55}[g.Core.Players[event.Player].Weapon])
			case "nova":
				g.sound.PlayEffect(57)
			case "explosion":
				g.sound.PlayEffect(29)
			case "hit":
				g.sound.PlayEffect(28)
			case "pickup":
				g.sound.PlayEffect(56)
			}
		}
	}
	return nil
}

func (g *Game) start() {
	g.Core.Start(g.players)
	g.paused = false
	g.CancelInput()
	if g.sound != nil {
		g.sound.UseMenu(false)
		g.sound.PlayTrack(1)
	}
}

// Start enters native gameplay, optionally selecting one of the decoded stages.
func (g *Game) Start(players, stage int) {
	g.players = max(1, min(2, players))
	g.start()
	if stage >= 0 && stage < len(g.Core.Data.Stages) {
		g.Core.Stage = stage
		if stage > 0 {
			g.Core.Campaign.ActiveCave = stage
			for _, gate := range g.Core.Campaign.Gates {
				if gate.Phase == stage {
					g.Core.Campaign.SurfaceScroll = gate.Progress + 100
				}
			}
		}
	}
}

func (g *Game) togglePause() {
	if g.Core.Mode != engine.Playing {
		return
	}
	if !g.paused {
		g.Pause()
		return
	}
	g.paused = false
	g.CancelInput()
	if g.audioPlayer != nil && !g.mute {
		g.audioPlayer.Play()
	}
}

func (g *Game) updateOptions() {
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowUp) {
		g.selectedOption = (g.selectedOption + 4) % 5
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowDown) {
		g.selectedOption = (g.selectedOption + 1) % 5
	}
	delta := 0
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		delta = -1
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) || inpututil.IsKeyJustPressed(ebiten.KeySpace) {
		delta = 1
	}
	o := &g.Core.Options
	switch g.selectedOption {
	case 0:
		o.Lives = max(3, min(5, o.Lives+delta))
	case 1:
		o.EnemyProjectileCap = max(1, min(12, o.EnemyProjectileCap+delta))
	case 2:
		o.EnemyProjectileSpeed = max(1, min(5, o.EnemyProjectileSpeed+delta))
	case 3:
		o.EnemyFireDelay = max(8, min(120, o.EnemyFireDelay+delta*4))
	case 4:
		o.StartWeapon = max(0, min(3, o.StartWeapon+delta))
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) {
		g.optionsOpen = false
		g.saveSettings()
	}
}

// Layout retains original pixels and adds separate touch gutters on mobile.
func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	if g.touchEnabled {
		return 480, 256
	}
	return Width, Height
}

func (g *Game) saveCapture(screen *ebiten.Image) {
	if g.Capture == "" || g.captured || g.updates < g.SmokeFrames {
		return
	}
	w, h := screen.Bounds().Dx(), screen.Bounds().Dy()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	screen.ReadPixels(img.Pix)
	if err := os.MkdirAll(filepath.Dir(g.Capture), 0755); err != nil {
		panic(err)
	}
	file, err := os.Create(g.Capture)
	if err != nil {
		panic(err)
	}
	if err := png.Encode(file, img); err != nil {
		file.Close()
		panic(err)
	}
	if err := file.Close(); err != nil {
		panic(err)
	}
	g.captured = true
}

var white = color.RGBA{255, 255, 255, 255}
var gold = color.RGBA{255, 221, 85, 255}

func (g *Game) windowWidth() int {
	if g.touchEnabled {
		return 480
	}
	return 320
}

func platformExit() bool { return runtime.GOOS != "android" }

func scoreText(score int) string { return fmt.Sprintf("%08d", score) }
