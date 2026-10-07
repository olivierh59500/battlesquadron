//go:build android

// Package mobile attaches the shared Go game to Ebitengine's Android view.
package mobile

import (
	"encoding/json"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/game"
	"github.com/olivierh59500/battlesquadron/internal/performance"
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
	performanceCommand  atomic.Uint32
	renderCommand       atomic.Uint32
	performanceState    atomic.Pointer[string]
	monitor             *performance.Monitor
	measurementReady    time.Time
	motionStart         int
	motionStarted       bool
}

var gameHost host

func init() {
	// Battle Squadron updates at the original Amiga PAL cadence.
	ebiten.SetTPS(50)
	ebiten.SetVsyncEnabled(true)
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
	if gameHost.verify.Load() && scene >= 1 && scene <= 5 {
		gameHost.scene.Store(int32(scene))
	}
}

// StartPerformanceMeasurement queues a warmup followed by real Update/Draw timings.
func StartPerformanceMeasurement() {
	if gameHost.verify.Load() {
		gameHost.performanceCommand.Store(1)
	}
}

// FinishPerformanceMeasurement publishes diagnostics outside normal rendering.
func FinishPerformanceMeasurement() {
	if gameHost.verify.Load() {
		gameHost.performanceCommand.Store(2)
	}
}

// PerformanceState returns the update-thread report used by emulator checks.
func PerformanceState() string {
	if state := gameHost.performanceState.Load(); state != nil {
		return *state
	}
	return "{}"
}

// SetVerificationSmoothRendering selects a diagnostics-only comparison mode.
func SetVerificationSmoothRendering(enabled bool) {
	if !gameHost.verify.Load() {
		return
	}
	command := uint32(2)
	if enabled {
		command = 1
	}
	gameHost.renderCommand.Store(command)
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
	start := time.Now()
	defer func() {
		if h.monitor != nil && h.game != nil {
			core := h.game.Core
			h.monitor.RecordUpdate(start, len(core.Enemies), len(core.PlayerShots), len(core.EnemyShots), len(core.Explosions)+len(core.NovaRays))
		}
	}()
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
		h.game.StopDemo()
		h.game.Start(1, 0)
		core := h.game.Core
		core.Options.Invulnerable = true
		core.Players[0].X, core.Players[0].Y, core.Players[0].Respawn = 128, 176, 0
		core.Frame = 1024
		if scene == 4 {
			core.Start(2)
			core.Options.Invulnerable = true
			for field := 0; field < 1000; field++ {
				core.Tick([2]engine.Input{{Fire: true}, {Fire: true}})
			}
			h.game.SmokeFrames = 180000
			h.game.SmokeNova = true
		} else if scene == 3 {
			if err := core.StartFinalBattle(); err != nil {
				return err
			}
			core.Scroll = 240
		} else {
			core.Stage, core.Scroll = int(scene), 2601
			if scene == 5 {
				core.Stage = 3
			}
		}
		for index := range core.Players {
			core.Players[index].Level = 5
		}
	}
	if command := h.renderCommand.Swap(0); command != 0 && h.verify.Load() {
		h.game.SetSmoothRendering(command == 1)
	}
	if command := h.performanceCommand.Swap(0); command != 0 && h.verify.Load() {
		if command == 1 {
			h.monitor = performance.New(2 * time.Second)
			h.measurementReady, h.motionStarted = time.Now().Add(2*time.Second), false
			state := "{}"
			h.performanceState.Store(&state)
		} else if h.monitor != nil {
			_, changes, density := h.game.RenderingMotion()
			report := struct {
				performance.Report
				PhysicalDensity      int     `json:"viewport_density"`
				TerrainMotionChanges int     `json:"terrain_motion_changes"`
				TerrainMotionHz      float64 `json:"terrain_motion_hz"`
			}{Report: h.monitor.Snapshot(ebiten.ActualFPS(), ebiten.ActualTPS()), PhysicalDensity: density, TerrainMotionChanges: changes - h.motionStart}
			if report.ElapsedSeconds > 0 {
				report.TerrainMotionHz = float64(report.TerrainMotionChanges) / report.ElapsedSeconds
			}
			data, err := json.Marshal(report)
			if err != nil {
				return err
			}
			state := string(data)
			h.performanceState.Store(&state)
			h.monitor = nil
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
			Demo, DemoInputBlocked                                                    bool
			MenuIdleTicks                                                             int
		}{core.Frame, core.Stage, core.Scroll, core.Players[0].X, core.Players[0].Y, len(core.PlayerShots), h.fired, int(core.Mode), core.Players[0].Respawn, core.NovaFrames, core.Players[0].Nova,
			h.game.DemoActive(), h.game.DemoInputBlocked(), h.game.MenuIdleTicks()})
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
		start := time.Now()
		if h.monitor != nil && !h.motionStarted && !start.Before(h.measurementReady) {
			_, h.motionStart, _ = h.game.RenderingMotion()
			h.motionStarted = true
		}
		h.game.Draw(screen)
		if h.monitor != nil {
			h.monitor.RecordDraw(start)
		}
	}
}

// DrawFinalScreen keeps the shared renderer's interpolated physical pixels.
func (h *host) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	if h.game != nil {
		h.game.DrawFinalScreen(screen, offscreen, geoM)
	}
}

func (h *host) Layout(width, height int) (int, int) {
	if h.game != nil {
		return h.game.Layout(width, height)
	}
	return 480, 256
}
