// Command attractcheck observes the real idle-menu demonstration without input injection.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/game"
	"github.com/olivierh59500/battlesquadron/internal/performance"
	"github.com/olivierh59500/battlesquadron/internal/replay"
)

type report struct {
	Success               bool               `json:"success"`
	Error                 string             `json:"error,omitempty"`
	Touch                 bool               `json:"touch_preview"`
	Audio                 bool               `json:"audio"`
	IdleStartTicks        int                `json:"idle_start_ticks"`
	IdleStartSeconds      float64            `json:"idle_start_seconds"`
	MaxIdleTicks          int                `json:"max_idle_ticks"`
	FocusedUpdates        int                `json:"focused_updates"`
	UnfocusedUpdates      int                `json:"unfocused_updates"`
	ElapsedSeconds        float64            `json:"elapsed_seconds"`
	CoreFrame             int                `json:"core_frame"`
	DemoActive            bool               `json:"demo_active"`
	Invulnerable          bool               `json:"invulnerable"`
	OrdinaryOptions       bool               `json:"ordinary_options"`
	ForecastingInRuntime  bool               `json:"forecasting_in_runtime"`
	PhysicalInputInjected bool               `json:"physical_input_injected"`
	SourceSHA256          string             `json:"source_sha256"`
	RecordingSHA256       string             `json:"recording_sha256"`
	StartX                int                `json:"start_x"`
	StartY                int                `json:"start_y"`
	StartScroll           int                `json:"start_scroll"`
	EndX                  int                `json:"end_x"`
	EndY                  int                `json:"end_y"`
	EndScroll             int                `json:"end_scroll"`
	NativeMovement        bool               `json:"native_movement"`
	TerrainMotionChanges  int                `json:"terrain_motion_changes"`
	PhysicalDensity       int                `json:"viewport_density"`
	Capture               string             `json:"capture"`
	Performance           performance.Report `json:"performance"`
}

type check struct {
	game        *game.Game
	monitor     *performance.Monitor
	started     time.Time
	deadline    time.Time
	fields      int
	capture     string
	seenDemo    bool
	motionStart int
	report      report
}

func (c *check) Update() error {
	if c.game.Capture != "" {
		if stat, err := os.Stat(c.capture); err == nil && stat.Size() > 0 {
			c.report.Success = true
			return ebiten.Termination
		}
	}
	if time.Now().After(c.deadline) {
		return fmt.Errorf("idle demonstration did not finish its observation before the deadline; keep the game focused and do not provide player input")
	}
	start := time.Now()
	idleBefore := c.game.MenuIdleTicks()
	c.report.MaxIdleTicks = max(c.report.MaxIdleTicks, idleBefore)
	if ebiten.IsFocused() {
		c.report.FocusedUpdates++
	} else {
		c.report.UnfocusedUpdates++
	}
	if err := c.game.Update(); err != nil {
		return err
	}
	core := c.game.Core
	active := c.game.DemoActive()
	if !c.seenDemo && active {
		c.seenDemo = true
		c.report.IdleStartTicks = idleBefore + 1
		c.report.IdleStartSeconds = time.Since(c.started).Seconds()
		c.report.StartX, c.report.StartY, c.report.StartScroll = core.Players[0].X, core.Players[0].Y, core.Scroll
		_, c.motionStart, _ = c.game.RenderingMotion()
		c.monitor = performance.New(0)
	}
	if !c.seenDemo {
		return nil
	}
	if !active {
		return fmt.Errorf("physical player activity interrupted the demonstration during observation")
	}
	c.monitor.RecordUpdate(start, len(core.Enemies), len(core.PlayerShots), len(core.EnemyShots), len(core.Explosions)+len(core.NovaRays))
	c.report.CoreFrame, c.report.DemoActive = core.Frame, active
	c.report.Invulnerable, c.report.OrdinaryOptions = core.Options.Invulnerable, core.Options == engine.DefaultOptions()
	c.report.EndX, c.report.EndY, c.report.EndScroll = core.Players[0].X, core.Players[0].Y, core.Scroll
	c.report.NativeMovement = c.report.StartX != c.report.EndX || c.report.StartY != c.report.EndY || c.report.StartScroll != c.report.EndScroll
	if core.Frame >= c.fields {
		if !c.report.OrdinaryOptions || !c.report.NativeMovement {
			return fmt.Errorf("the demonstration did not use ordinary options and native movement")
		}
		if c.game.Capture == "" {
			// Capture is armed only after ordinary Update has advanced the demo.
			// SmokeFrames stays zero, so the actual fifteen-second idle path runs.
			c.game.Capture = c.capture
		}
	}
	return nil
}

func (c *check) Draw(screen *ebiten.Image) {
	start := time.Now()
	c.game.Draw(screen)
	if c.monitor != nil {
		c.monitor.RecordDraw(start)
	}
}

func (c *check) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	c.game.DrawFinalScreen(screen, offscreen, geoM)
}

func (c *check) Layout(width, height int) (int, int) { return c.game.Layout(width, height) }

func main() {
	touch := flag.Bool("touch", false, "include the Android touch-control preview")
	mute := flag.Bool("mute", false, "disable audio; audio is included by default")
	fields := flag.Int("fields", 250, "ordinary demo PAL fields to observe; at least 200")
	deadline := flag.Duration("deadline", 35*time.Second, "wall-clock limit including the fifteen-second idle menu")
	output := flag.String("output", "captures/attract-desktop.json", "local JSON observation report")
	capture := flag.String("capture", "captures/attract-desktop.png", "actual Ebitengine frame capture")
	flag.Parse()
	if *fields < 200 || *deadline <= 0 {
		log.Fatal("fields must be at least 200 and deadline must be positive")
	}
	if err := os.Remove(*capture); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	g, err := game.New()
	if err != nil {
		log.Fatal(err)
	}
	g.SetTouchEnabled(*touch)
	g.SetMuted(*mute)
	g.SetDataDir(filepath.Join(".cache", "attractcheck"))
	metadata := replay.ExpertMetadata()
	started := time.Now()
	c := &check{game: g, started: started, deadline: started.Add(*deadline), fields: *fields, capture: *capture,
		report: report{Touch: *touch, Audio: !*mute, SourceSHA256: metadata.SourceSHA256, RecordingSHA256: metadata.RecordingSHA256, Capture: *capture}}
	width, height := g.Layout(0, 0)
	ebiten.SetWindowSize(width*3, height*3)
	ebiten.SetWindowTitle("Battle Squadron idle demonstration check")
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetTPS(50)
	ebiten.SetVsyncEnabled(true)
	runErr := ebiten.RunGame(c)
	c.report.ElapsedSeconds = time.Since(started).Seconds()
	if runErr != nil {
		c.report.Error = runErr.Error()
	} else if !c.report.Success {
		c.report.Error = "the game closed before the idle demonstration was captured"
	}
	if c.monitor != nil {
		c.report.Performance = c.monitor.Snapshot(ebiten.ActualFPS(), ebiten.ActualTPS())
		_, changes, density := g.RenderingMotion()
		c.report.TerrainMotionChanges, c.report.PhysicalDensity = changes-c.motionStart, density
	}
	if err := writeReport(*output, c.report); err != nil {
		log.Fatal(err)
	}
	if !c.report.Success {
		log.Fatal(c.report.Error)
	}
	fmt.Printf("Idle menu started the expert demo at %d PAL ticks; captured field %d with %.2f FPS; report %s\n", c.report.IdleStartTicks, c.report.CoreFrame, c.report.Performance.MeasuredFPS, *output)
}

func writeReport(name string, value report) error {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return err
	}
	file, err := os.Create(name)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}
