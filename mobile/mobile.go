//go:build android

// Package mobile attaches the shared Go game to Ebitengine's Android view.
package mobile

import (
	"encoding/json"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"
	"github.com/olivierh59500/battlesquadron/internal/game"
)

const (
	pauseCommand uint32 = 1 << iota
	backCommand
	cancelCommand
)

// Android callbacks only publish requests; the update thread owns game state.
type host struct {
	game                *game.Game
	directory           atomic.Pointer[string]
	commands            atomic.Uint32
	atTitle             atomic.Bool
	verify              atomic.Bool
	snapshot            atomic.Pointer[string]
	fired, sampledFrame int
	scene               atomic.Int32
}

var gameHost host

func init() {
	// Battle Squadron updates at the original Amiga PAL cadence.
	ebiten.SetTPS(50)
	ebiten.SetScreenFilterEnabled(false)
	gameHost.atTitle.Store(true)
	enginemobile.SetGame(&gameHost)
}

// Configure supplies Android private storage before the first game update.
func Configure(directory string) { gameHost.directory.Store(&directory) }

// RequestPause preserves gameplay after Android suspends the activity.
func RequestPause() { gameHost.commands.Or(pauseCommand | cancelCommand) }

// RequestBack routes Android's system Back gesture through the game.
func RequestBack() { gameHost.commands.Or(backCommand | cancelCommand) }

// CancelInput releases unfinished gestures after a focus or surface change.
func CancelInput() { gameHost.commands.Or(cancelCommand) }

// IsAtTitle lets the native activity finish when Back is pressed at the title.
func IsAtTitle() bool { return gameHost.atTitle.Load() }

// SetVerificationEnabled enables snapshots for the generated emulator checks.
func SetVerificationEnabled(enabled bool) { gameHost.verify.Store(enabled) }

// SetVerificationScene queues a fixture only while emulator diagnostics are enabled.
func SetVerificationScene(scene int) {
	if gameHost.verify.Load() && scene >= 1 && scene <= 3 {
		gameHost.scene.Store(int32(scene))
	}
}

// VerificationState returns a snapshot published by the Go update thread.
func VerificationState() string {
	if snapshot := gameHost.snapshot.Load(); snapshot != nil {
		return *snapshot
	}
	return "{}"
}

// Dummy keeps a stable exported symbol available to the binding generator.
func Dummy() {}

func (h *host) Update() error {
	if h.game == nil {
		directory := h.directory.Load()
		if directory == nil {
			return nil
		}
		// EbitenView must establish its JNI context before graphics or audio exist.
		var err error
		h.game, err = game.New()
		if err != nil {
			return err
		}
		h.game.SetTouchEnabled(true)
		if storage, ok := any(h.game).(interface{ SetDataDir(string) }); ok {
			storage.SetDataDir(*directory)
		}
	}
	commands := h.commands.Swap(0)
	if commands&cancelCommand != 0 {
		h.game.CancelInput()
	}
	if commands&pauseCommand != 0 {
		h.game.Pause()
	}
	if commands&backCommand != 0 {
		h.game.Back()
	}
	if scene := h.scene.Swap(0); scene != 0 && h.verify.Load() {
		core := h.game.Core
		core.Start(1)
		core.Options.Invulnerable = true
		core.Players[0].X, core.Players[0].Y, core.Players[0].Respawn = 128, 176, 0
		core.Frame = 1024
		if scene == 3 {
			if err := core.StartFinalBattle(); err != nil {
				return err
			}
			core.Scroll = 240
		} else {
			core.Stage, core.Scroll = int(scene), 2601
		}
	}
	if err := h.game.Update(); err != nil {
		return err
	}
	h.atTitle.Store(h.game.AtTitle())
	if h.verify.Load() {
		core := h.game.Core
		if core.Frame != h.sampledFrame {
			for _, event := range core.Events {
				if event.Kind == "shot" && event.Player == 0 {
					h.fired++
				}
			}
			h.sampledFrame = core.Frame
		}
		data, err := json.Marshal(struct {
			Frame, Stage, Scroll, X, Y, Shots, Fired, Mode, Respawn, NovaFrames, Nova int
		}{core.Frame, core.Stage, core.Scroll, core.Players[0].X, core.Players[0].Y, len(core.PlayerShots), h.fired, int(core.Mode), core.Players[0].Respawn, core.NovaFrames, core.Players[0].Nova})
		if err != nil {
			return err
		}
		state := string(data)
		h.snapshot.Store(&state)
	}
	return nil
}

func (h *host) Draw(screen *ebiten.Image) {
	if h.game != nil {
		h.game.Draw(screen)
	}
}

func (h *host) Layout(width, height int) (int, int) {
	if h.game != nil {
		return h.game.Layout(width, height)
	}
	return 480, 256
}
