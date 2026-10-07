// Battle Squadron runs the native Go game with its locally extracted original assets.
package main

import (
	"flag"
	"log"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/battlesquadron/internal/game"
)

func main() {
	touch := flag.Bool("touch", false, "preview Android touch controls")
	originalCadence := flag.Bool("original-cadence", false, "render original 25/50 Hz steps without smooth interpolation")
	mute := flag.Bool("mute", false, "disable audio")
	play := flag.Bool("play", false, "start directly in the native game")
	screen := flag.String("screen", "game", "smoke capture screen: game or title")
	players := flag.Int("players", 1, "one or two simultaneous players")
	stage := flag.Int("stage", 0, "start on original surface (0) or underground stage (1-3)")
	final := flag.Bool("final", false, "start the original final encounter for inspection")
	nova := flag.Bool("nova", false, "activate an original Nova during a smoke run")
	smoke := flag.Int("smoke", 0, "drive native gameplay for this many PAL ticks, then exit")
	capture := flag.String("capture", "", "write the actual final Ebitengine frame to PNG")
	scale := flag.Int("scale", 3, "integer desktop window scale")
	fullscreen := flag.Bool("fullscreen", false, "open in fullscreen")
	dataDir := flag.String("data-dir", "", "directory for local settings and scores")
	flag.Parse()
	if *screen != "game" && *screen != "title" {
		log.Fatal("-screen must be game or title")
	}
	g, err := game.New()
	if err != nil {
		log.Fatal(err)
	}
	g.SetTouchEnabled(*touch)
	g.SetSmoothRendering(!*originalCadence)
	g.SetMuted(*mute)
	g.SetDataDir(*dataDir)
	g.SmokeFrames, g.Capture = *smoke, *capture
	g.SmokeNova = *nova
	if *play || *smoke > 0 && *screen == "game" || *stage != 0 {
		g.Start(*players, *stage)
	}
	if *final {
		g.Start(*players, 0)
		if err := g.Core.StartFinalBattle(); err != nil {
			log.Fatal(err)
		}
	}
	width, height := g.Layout(0, 0)
	ebiten.SetWindowSize(width*max(1, *scale), height*max(1, *scale))
	ebiten.SetWindowTitle("Battle Squadron — Go / Ebitengine")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetTPS(50)
	ebiten.SetVsyncEnabled(true)
	ebiten.SetFullscreen(*fullscreen)
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
