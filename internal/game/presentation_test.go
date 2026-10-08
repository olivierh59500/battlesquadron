package game

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

func presentationFixture() *Game {
	g := &Game{Core: &engine.Engine{Mode: engine.Playing}, presentation: newPresentation()}
	for frame := 1; frame <= 6; frame++ {
		g.Core.Frame = frame
		g.Core.Scroll = (frame + 1) / 2
		g.Core.Players[0] = engine.Player{X: 2 * frame, Y: 100, Active: true, Lives: 3}
		g.Core.Enemies = []engine.Enemy{{ID: 7, X: (frame + 1) / 2 * 4, Y: 50, Age: (frame + 1) / 2}}
		g.Core.PlayerShots = []engine.Bullet{{Player: 0, Slot: 1, Graphic: 4, X: 100, Y: 100 - 6*frame, VY: -6, Age: frame}}
		g.observePresentation()
	}
	return g
}

func TestRecordingClockKeepsIntermediatePositionsIndependentOfWallTime(t *testing.T) {
	g := &Game{Core: &engine.Engine{Mode: engine.Playing}, presentation: newPresentation()}
	base := time.Unix(100, 0)
	for frame := 1; frame <= 5; frame++ {
		g.Core.Frame, g.Core.Scroll = frame, (frame+1)/2
		g.Core.Players[0] = engine.Player{X: frame * 2, Y: 100, Active: true, Lives: 3}
		g.observePresentationAt(base.Add(time.Duration(frame) * palField))
	}
	g.Core.Frame, g.Core.Scroll = 6, 3
	g.Core.Players[0].X = 12
	before := g.Core.Digest()
	stamp := base.Add(6*palField + palField/2)
	g.preparePresentation(stamp)
	if g.presentation.tickTime != base.Add(6*palField) || g.presentation.scroll != 2.75 {
		t.Fatal("offline recording inherited wall time instead of its PAL timestamp")
	}
	x, _ := g.presentation.player(0, g.Core.Players[0])
	if x != 9 || g.Core.Digest() != before {
		t.Fatal("virtual presentation clock changed simulation or lost intermediate motion")
	}
}

func TestPresentationSmoothsBothNativeCadencesWithoutMutatingEngine(t *testing.T) {
	g := presentationFixture()
	originalPlayers := g.Core.Players
	originalEnemies := append([]engine.Enemy(nil), g.Core.Enemies...)
	originalShots := append([]engine.Bullet(nil), g.Core.PlayerShots...)
	g.presentation.tickTime = time.Now()
	g.preparePresentation(g.presentation.tickTime.Add(palField / 2))
	if math.Abs(g.presentation.scroll-2.75) > 1e-9 {
		t.Fatalf("25 Hz terrain should interpolate between odd PAL fields: %.6f", g.presentation.scroll)
	}
	px, _ := g.presentation.player(0, g.Core.Players[0])
	ex, _ := g.presentation.enemy(g.Core.Enemies[0])
	_, sy := g.presentation.shot(g.Core.PlayerShots[0], true, g.Core.Frame)
	if px != 9 || ex != 11 || sy != 73 {
		t.Fatalf("mixed-cadence positions: player %.3f enemy %.3f projectile %.3f", px, ex, sy)
	}
	if g.Core.Frame != 6 || g.Core.Scroll != 3 || g.Core.Players != originalPlayers || !reflect.DeepEqual(g.Core.Enemies, originalEnemies) || !reflect.DeepEqual(g.Core.PlayerShots, originalShots) {
		t.Fatal("presentation changed the deterministic native simulation")
	}
}

func TestPresentationRetainsFractionalMovementOnRepeatedDraws(t *testing.T) {
	g := presentationFixture()
	g.presentation.tickTime = time.Now()
	g.preparePresentation(g.presentation.tickTime.Add(palField / 4))
	first := g.presentation.scroll
	g.preparePresentation(g.presentation.tickTime.Add(palField * 3 / 4))
	second := g.presentation.scroll
	if second-first != 0.25 || g.Core.Scroll != 3 || g.Core.Frame != 6 {
		t.Fatalf("Draw should move between unchanged native fields: %.3f -> %.3f", first, second)
	}
}

func TestPeriodicInterpolationKeepsEarlyPALDispatchContinuous(t *testing.T) {
	g := presentationFixture()
	anchor := time.Now()
	g.presentation.tickTime = anchor
	g.preparePresentation(anchor.Add(-palField / 4))
	first := g.presentation.scroll
	g.Core.Frame = 7
	g.Core.Scroll = 4
	g.observePresentation()
	g.presentation.tickTime = anchor.Add(palField)
	g.preparePresentation(anchor.Add(palField / 4))
	if first != 2.375 || g.presentation.scroll != 2.625 {
		t.Fatalf("an early Update quantized smooth motion: %.3f -> %.3f", first, g.presentation.scroll)
	}
}

func TestHostileShotsUseTheContinuousTerrainCamera(t *testing.T) {
	g := &Game{Core: &engine.Engine{Mode: engine.Playing}, presentation: newPresentation()}
	for frame := 1; frame <= 6; frame++ {
		g.Core.Frame, g.Core.CameraX = frame, (frame+1)/2
		g.Core.EnemyShots = []engine.Bullet{{X: 100 + 4*frame - g.Core.CameraX, Y: 50, VX: 4 << 16, Graphic: 2, Age: frame}}
		g.observePresentation()
	}
	g.presentation.tickTime = time.Now()
	g.preparePresentation(g.presentation.tickTime.Add(palField / 2))
	x, y := g.presentation.shot(g.Core.EnemyShots[0], false, g.Core.Frame)
	if x != 115.25 || y != 50 {
		t.Fatalf("50 Hz shot and 25 Hz camera drifted apart: %.3f, %.3f", x, y)
	}
}

func TestPresentationSnapsNewActorsTeleportsDeathAndStageChanges(t *testing.T) {
	g := presentationFixture()
	g.presentation.drawFrame = 4.5
	x, y := g.presentation.enemy(engine.Enemy{ID: 99, X: 140, Y: 150})
	if x != 140 || y != 150 {
		t.Fatal("a new actor interpolated from a reused pool slot")
	}
	x, y = safePosition(0, 0, 200, 200, 0.5, 200, 200)
	if x != 200 || y != 200 {
		t.Fatal("a teleport smeared across the playfield")
	}
	p := g.Core.Players[0]
	p.Lives--
	p.X = 200
	x, _ = g.presentation.player(0, p)
	if x != 200 {
		t.Fatal("a respawning player interpolated from its previous life")
	}
	g.Core.Frame++
	g.Core.Stage++
	g.Core.Scroll = 256
	g.observePresentation()
	if g.presentation.count != 1 {
		t.Fatal("a stage transition retained unrelated terrain history")
	}
}

func TestOriginalCadenceAndPauseRetainNativeCoordinates(t *testing.T) {
	g := presentationFixture()
	g.SetSmoothRendering(false)
	g.preparePresentation(time.Now())
	x, _ := g.presentation.player(0, g.Core.Players[0])
	if x != 12 || g.presentation.scroll != 3 {
		t.Fatal("original-cadence mode interpolated")
	}
	g = presentationFixture()
	g.paused = true
	g.preparePresentation(time.Now())
	x, _ = g.presentation.player(0, g.Core.Players[0])
	if x != 12 || g.presentation.scroll != 3 {
		t.Fatal("paused presentation kept moving")
	}
}
