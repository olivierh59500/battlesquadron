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
	game      *game.Game
	directory atomic.Pointer[string]
	commands  atomic.Uint32
	atTitle   atomic.Bool
	verify    atomic.Bool
	snapshot  atomic.Pointer[string]
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
	if err := h.game.Update(); err != nil {
		return err
	}
	h.atTitle.Store(h.game.AtTitle())
	if h.verify.Load() {
		core := h.game.Core
		data, err := json.Marshal(struct {
			Frame, Stage, Scroll, X, Y, Shots, Mode, Respawn int
		}{core.Frame, core.Stage, core.Scroll, core.Players[0].X, core.Players[0].Y, len(core.PlayerShots), int(core.Mode), core.Players[0].Respawn})
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
