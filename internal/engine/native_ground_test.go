package engine

import (
	"encoding/binary"
	"os"
	"testing"
)

func groundFixture(kind byte, mode int) (*Engine, Enemy) {
	loader := make([]byte, 0x7000)
	record := loader[0x100:0x130]
	binary.BigEndian.PutUint16(record[6:], 32)
	binary.BigEndian.PutUint16(record[8:], 2)
	record[17], record[19], record[28], record[33], record[34], record[35] = kind, 8, 2, 10, 7, 0
	binary.BigEndian.PutUint16(record[20:], 12)
	binary.BigEndian.PutUint16(record[22:], 16)
	// The actual $5F02 direction table is relocated to $5E68 in this loader.
	copy(loader[0x5d60:], []byte{0, 1, 1, 1, 1, 255, 255, 255, 0, 1, 1, 1, 1, 255, 255, 255})
	e := &Engine{Data: &Data{Loader: loader, LoaderBase: 0x100, Random: []byte{0}, Stages: []Stage{{Mode: mode, Width: 24, Height: 8192}}}, Options: DefaultOptions(), Players: [2]Player{{Active: true, Lives: 3, X: 100, Y: 80}, {Active: true, Lives: 3, X: 220, Y: 160}}}
	definition, _ := DecodeDefinition(loader, 0x100, 0x200)
	return e, Enemy{X: 100, Y: 80, Health: 2, Definition: definition}
}

func groundStep(e *Engine, enemy *Enemy, clock int) {
	e.Frame = clock + 1
	e.npcPhase = true
	e.Scroll++
	if !e.moveNativeGround(enemy) {
		panic("fixture was not decoded")
	}
}

func TestGroundDamageIsDelayedAndWreckKeepsOriginalFrames(t *testing.T) {
	e, enemy := groundFixture(32, 3)
	e.Enemies = []Enemy{enemy}
	if !e.damageNativeGround(0, 3, 0) {
		t.Fatal("ground mailbox did not intercept damage")
	}
	if e.Enemies[0].Health != 2 || e.Enemies[0].ground.damage != 3 {
		t.Fatal("collision applied damage before the original NPC pass")
	}
	enemy = e.Enemies[0]
	groundStep(e, &enemy, 2)
	if enemy.Frame != 8 || enemy.Health < 0 || e.nativeGroundTouchable(enemy) {
		t.Fatalf("ground death removed its original wreck state: %+v", enemy)
	}
	if e.Players[0].Score != enemy.Definition.Score {
		t.Fatal("death did not award the decoded original score once")
	}
	groundStep(e, &enemy, 4)
	if enemy.Frame != 9 {
		t.Fatal("first death step did not use clock bit one")
	}
	groundStep(e, &enemy, 6)
	if enemy.Frame != 9 {
		t.Fatal("death advanced on the excluded clock phase")
	}
	groundStep(e, &enemy, 8)
	if enemy.Frame != 10 {
		t.Fatal("wreck did not reach its original final frame")
	}
	e.Players[0].X, e.Players[0].Y = enemy.X-8, enemy.Y
	groundStep(e, &enemy, 10)
	if e.Players[0].WreckBonus != 1 || enemy.Frame != 11 {
		t.Fatal("finished wreck was not claimed using its original point box")
	}
	groundStep(e, &enemy, 12)
	if e.Players[0].WreckBonus != 1 {
		t.Fatal("wreck was collected twice")
	}
}

func TestGroundHitFlashPreservesDamagePhase(t *testing.T) {
	e, enemy := groundFixture(32, 3)
	e.Enemies = []Enemy{enemy}
	e.damageNativeGround(0, 1, 0)
	enemy = e.Enemies[0]
	groundStep(e, &enemy, 0)
	if enemy.Frame != 7 || enemy.ground.flash != 3 {
		t.Fatal("first hit did not use the full original odd flash count")
	}
	groundStep(e, &enemy, 2)
	if enemy.Frame != 0 || enemy.ground.flash != 2 {
		t.Fatal("even flash did not restore normal frame")
	}
	groundStep(e, &enemy, 4)
	if enemy.Frame != 7 || enemy.ground.flash != 1 {
		t.Fatal("odd flash did not select original hit frame")
	}
	groundStep(e, &enemy, 6)
	if enemy.Frame != 0 || enemy.ground.flash != 0 {
		t.Fatal("flash did not end on normal state")
	}
}

func TestOriginalGroundCollisionBounds(t *testing.T) {
	e, enemy := groundFixture(32, 0)
	e.initializeGround(&enemy)
	x, y, w, h := e.GroundCollisionBounds(enemy)
	if x != 112 || y != 80 || w != 16 || h != 32 {
		t.Fatalf("ground hit box became the full graphic rectangle: %d,%d %dx%d", x, y, w, h)
	}
}

func TestOpeningLauncherRisesAndEmitsOnlyOneOriginalBeam(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if err != nil {
		t.Skip("original loader is locally extracted")
	}
	var definition Definition
	for _, rule := range DecodeMapRules(loader, 0x100) {
		if rule.Mode == 0 && rule.Definition.Kind == 1 {
			definition = rule.Definition
			break
		}
	}
	if definition.Address == 0 {
		t.Fatal("opening launcher definition is missing")
	}
	e := &Engine{Data: &Data{Loader: loader, LoaderBase: 0x100, Random: []byte{0}, Stages: []Stage{{Mode: 0, Width: 24, Height: 8192}}}, Options: DefaultOptions(), Players: [2]Player{{Active: true, Lives: 3}, {Active: true, Lives: 3}}}
	enemy := Enemy{X: 60, Y: -1, Health: definition.Health, Definition: definition}
	for clock := 2; clock < 90; clock += 2 {
		groundStep(e, &enemy, clock)
	}
	spawns := e.takeNativeGroundSpawns()
	if len(spawns) != 1 || spawns[0].Definition.NativeKind != 7 || !spawns[0].Definition.FlyingPool {
		t.Fatalf("launcher emitted %d non-original beams", len(spawns))
	}
	if enemy.Frame != 9 || enemy.ground.counterA != 255 || e.nativeGroundTouchable(enemy) {
		t.Fatal("launcher did not remain open and invulnerable after its one shot")
	}
	if len(e.takeNativeGroundSpawns()) != 0 {
		t.Fatal("ground allocation queue was not drained")
	}
}

func TestRotatingTurretCannotFireBeforeTurningAndSettling(t *testing.T) {
	e, enemy := groundFixture(32, 0)
	e.Players[0].X, e.Players[0].Y = 250, enemy.Y+6
	e.Players[1].Active = false
	groundStep(e, &enemy, 0)
	if enemy.Frame != 1 || enemy.ground.counterA != 10 || len(e.EnemyShots) != 0 {
		t.Fatal("turret skipped its one-direction turn and ten-count pause")
	}
	for n := 1; n <= 10; n++ {
		e.Players[0].Y = enemy.Y + 7
		groundStep(e, &enemy, n*2)
		if enemy.Frame != 1 || len(e.EnemyShots) != 0 {
			t.Fatal("turret moved or fired during its original turn pause")
		}
	}
	e.Players[0].Y = enemy.Y + 7
	groundStep(e, &enemy, 22)
	if enemy.Frame != 2 || enemy.ground.counterA != 10 {
		t.Fatal("turret did not reach its next original direction")
	}
}

func TestGrowingBeamUsesIndividualPlaneBytesAndOriginalHeight(t *testing.T) {
	e := &Engine{Data: &Data{Stages: []Stage{{Mode: 0, Width: 24, Height: 256, Tiles: make([]uint16, 24*16), TileBank: make([]byte, 160)}}}, Options: DefaultOptions()}
	e.Scroll = 256
	if e.groundTerrainSolid(2, 1) {
		t.Fatal("zero terrain planes blocked a beam")
	}
	for i := range e.Data.Stages[0].TileBank {
		e.Data.Stages[0].TileBank[i] = 255
	}
	if e.groundTerrainSolid(2, 1) {
		t.Fatal("uniform full terrain planes blocked a beam")
	}
	e.Data.Stages[0].TileBank[30] = 1
	if !e.groundTerrainSolid(2, 255) {
		t.Fatal("nonuniform original plane byte failed to stop a beam")
	}
	for i := range e.Data.Stages[0].TileBank {
		e.Data.Stages[0].TileBank[i] = 0
	}
	e.Scroll = 0
	enemy := Enemy{X: 20, Y: 0, Health: 7, Definition: Definition{FlyingPool: true, NativeKind: 7, Width: 32, Height: 32}}
	for n := 0; n < 40; n++ {
		e.Scroll++
		e.Frame = n*2 + 1
		e.npcPhase = true
		e.moveNativeGroundBeam(&enemy)
	}
	if enemy.Definition.Height != 32 || enemy.Y != 0 {
		t.Fatalf("beam did not grow against empty terrain: height%d y%d", enemy.Definition.Height, enemy.Y)
	}
	enemy.ground.damage = 8
	e.Frame = 83
	e.Scroll++
	e.moveNativeGroundBeam(&enemy)
	if enemy.Frame != 3 || enemy.Definition.Sprite != "flying_10_0_stage_0" || enemy.Health < 0 {
		t.Fatal("beam skipped its original eight-state death animation")
	}
}

func TestWreckBonusCapsAtOriginalNinetyNine(t *testing.T) {
	e, enemy := groundFixture(32, 3)
	e.initializeGround(&enemy)
	enemy.ground.frame = enemy.ground.template.FinalFrame
	enemy.ground.flags = 4
	e.Players[0].X, e.Players[0].Y, e.Players[0].WreckBonus = enemy.X-8, enemy.Y, 99
	e.Players[1].Active = false
	e.claimGroundWreck(&enemy, 3)
	if e.Players[0].WreckBonus != 99 || enemy.ground.frame != enemy.ground.template.FinalFrame {
		t.Fatal("capped original bonus counter claimed another wreck")
	}
}
