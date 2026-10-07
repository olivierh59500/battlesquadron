// Command benchmark measures the real Ebitengine loop with original game assets.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/internal/engine"
	"github.com/olivierh59500/battlesquadron/internal/game"
	"github.com/olivierh59500/battlesquadron/internal/performance"
)

type benchmark struct {
	game                                  *game.Game
	monitor                               *performance.Monitor
	deadline                              time.Time
	measured                              time.Time
	motionStartFrames, motionStartChanges int
	motionStarted                         bool
}

func (b *benchmark) Update() error {
	if time.Now().After(b.deadline) {
		return ebiten.Termination
	}
	start := time.Now()
	err := b.game.Update()
	c := b.game.Core
	b.monitor.RecordUpdate(start, len(c.Enemies), len(c.PlayerShots), len(c.EnemyShots), len(c.Explosions)+len(c.NovaRays))
	return err
}

func (b *benchmark) Draw(screen *ebiten.Image) {
	start := time.Now()
	if !b.motionStarted && !start.Before(b.measured) {
		b.motionStartFrames, b.motionStartChanges, _ = b.game.RenderingMotion()
		b.motionStarted = true
	}
	b.game.Draw(screen)
	b.monitor.RecordDraw(start)
}

func (b *benchmark) DrawFinalScreen(screen ebiten.FinalScreen, offscreen *ebiten.Image, geoM ebiten.GeoM) {
	b.game.DrawFinalScreen(screen, offscreen, geoM)
}

func (b *benchmark) Layout(width, height int) (int, int) { return b.game.Layout(width, height) }

func main() {
	duration := flag.Duration("duration", 20*time.Second, "measured time after warmup")
	warmup := flag.Duration("warmup", 3*time.Second, "exclude initial uploads and scheduler stabilization")
	stage := flag.Int("stage", 0, "original stage to render (0-3)")
	final := flag.Bool("final", false, "measure the original final encounter")
	touch := flag.Bool("touch", false, "include Android touch-control rendering")
	mute := flag.Bool("mute", false, "exclude audio (audio is included by default)")
	unlimited := flag.Bool("unlimited", false, "disable VSync to measure desktop throughput")
	originalCadence := flag.Bool("original-cadence", false, "compare unchanged 25/50 Hz presentation without interpolation")
	prewarm := flag.Int("prewarm", 1000, "advance this many native fields before measuring")
	output := flag.String("output", "captures/performance-desktop.json", "local JSON report")
	cpuProfile := flag.String("cpu-profile", "", "optional local CPU profile")
	heapProfile := flag.String("heap-profile", "", "optional local allocation profile")
	flag.Parse()
	if *duration <= 0 || *warmup < 0 || *stage < 0 || *stage > 3 || *prewarm < 0 {
		log.Fatal("duration must be positive, warmup/prewarm nonnegative, and stage between 0 and 3")
	}
	created := time.Now()
	g, err := game.New()
	if err != nil {
		log.Fatal(err)
	}
	startup := time.Since(created)
	g.SetTouchEnabled(*touch)
	g.SetMuted(*mute)
	g.SetSmoothRendering(!*originalCadence)
	g.SetDataDir(filepath.Join(".cache", "benchmark"))
	g.Start(2, *stage)
	g.Core.Options.Invulnerable = true
	for field := 0; field < *prewarm; field++ {
		g.Core.Tick([2]engine.Input{{Fire: true}, {Fire: true}})
	}
	if *final {
		g.Start(2, 0)
		g.Core.Options.Invulnerable = true
		if err := g.Core.StartFinalBattle(); err != nil {
			log.Fatal(err)
		}
	}
	for index := range g.Core.Players {
		g.Core.Players[index].Level = 5
	}
	// The existing smoke driver supplies movement and firing through real Update.
	g.SmokeFrames = int((*duration+*warmup+10*time.Second).Seconds()*50) + 1
	g.SmokeNova = true
	var cpuFile *os.File
	if *cpuProfile != "" {
		cpuFile, err = create(*cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		if err := pprof.StartCPUProfile(cpuFile); err != nil {
			log.Fatal(err)
		}
	}
	b := &benchmark{game: g, monitor: performance.New(*warmup), deadline: time.Now().Add(*duration + *warmup), measured: time.Now().Add(*warmup)}
	width, height := g.Layout(0, 0)
	ebiten.SetWindowSize(width*3, height*3)
	ebiten.SetWindowTitle("Battle Squadron rendering benchmark")
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetTPS(50)
	ebiten.SetVsyncEnabled(!*unlimited)
	if err := ebiten.RunGame(b); err != nil {
		log.Fatal(err)
	}
	if cpuFile != nil {
		pprof.StopCPUProfile()
		if err := cpuFile.Close(); err != nil {
			log.Fatal(err)
		}
	}
	report := struct {
		performance.Report
		StartupMS            float64 `json:"startup_ms"`
		Stage                int     `json:"stage"`
		Final                bool    `json:"final"`
		Touch                bool    `json:"touch"`
		Audio                bool    `json:"audio"`
		VSync                bool    `json:"vsync"`
		Smooth               bool    `json:"smooth"`
		PhysicalDensity      int     `json:"viewport_density"`
		TerrainMotionChanges int     `json:"terrain_motion_changes"`
		TerrainMotionHz      float64 `json:"terrain_motion_hz"`
	}{Report: b.monitor.Snapshot(ebiten.ActualFPS(), ebiten.ActualTPS()), StartupMS: float64(startup) / float64(time.Millisecond), Stage: *stage, Final: *final, Touch: *touch, Audio: !*mute, VSync: !*unlimited, Smooth: !*originalCadence}
	_, changes, density := g.RenderingMotion()
	report.PhysicalDensity, report.TerrainMotionChanges = density, changes-b.motionStartChanges
	if report.ElapsedSeconds > 0 {
		report.TerrainMotionHz = float64(report.TerrainMotionChanges) / report.ElapsedSeconds
	}
	file, err := create(*output)
	if err != nil {
		log.Fatal(err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		log.Fatal(err)
	}
	if err := file.Close(); err != nil {
		log.Fatal(err)
	}
	if *heapProfile != "" {
		file, err := create(*heapProfile)
		if err != nil {
			log.Fatal(err)
		}
		runtime.GC()
		if err := pprof.Lookup("allocs").WriteTo(file, 0); err != nil {
			log.Fatal(err)
		}
		if err := file.Close(); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Printf("%.2f FPS, %.2f PAL updates/s; draw p95 %.3f ms, update p95 %.3f ms; report %s\n", report.MeasuredFPS, report.MeasuredTPS, report.DrawCPU.P95MS, report.UpdateCPU.P95MS, *output)
}

func create(name string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		return nil, err
	}
	return os.Create(name)
}
