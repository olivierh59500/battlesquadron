package engine

import (
	"os"
	"testing"
)

func fixtureData() *Data {
	data := &Data{Options: DefaultOptions()}
	for weapon := range data.Weapons {
		for level := range data.Weapons[weapon] {
			data.Weapons[weapon][level] = Weapon{Cooldown: 1, Width: 8, Height: 8, BankSize: 2,
				Slots: [12]int8{1, 1, 0, 0, -1, -1, -1, -1, -1, -1, -1, -1},
				Shots: []Shot{{X: 6, Y: -9, VX: -1, VY: -8, Height: 8, Graphic: 0x40, Delay: 1, Damage: 1},
					{X: 18, Y: -9, VX: 1, VY: -8, Height: 8, Graphic: 0x40, Delay: 1, Damage: 1}}}
		}
	}
	return data
}

func newFixture(t *testing.T) *Engine {
	t.Helper()
	engine, err := New(fixtureData())
	if err != nil {
		t.Fatal(err)
	}
	engine.Start(1)
	// Isolated movement/fire checks begin after the original entry animation.
	engine.Players[0].Respawn, engine.Players[0].Invulnerable, engine.Players[0].Y = 0, 0, 176
	engine.Scroll = 0
	return engine
}

func TestOriginalLoaderWeaponTable(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if os.IsNotExist(err) {
		t.Skip("extract the original ADF to validate its immutable weapon table")
	}
	if err != nil {
		t.Fatal(err)
	}
	weapons, table, err := DecodeWeapons(loader, 0x100, 0)
	if err != nil {
		t.Fatal(err)
	}
	if table != 0x1fc8 {
		t.Fatalf("actual loader table = $%x; want $1fc8", table)
	}
	base := weapons[0][0]
	if base.Address != 0x371a || base.Cooldown != 1 || base.Width != 8 || base.Height != 8 || len(base.Shots) != 2 {
		t.Fatalf("unexpected original base weapon: %+v", base)
	}
	if base.Shots[0].VX != -1 || base.Shots[0].VY != -6 || base.Shots[0].Graphic != 0x40 || base.Shots[1].VX != 1 {
		t.Fatalf("unexpected original base projectiles: %+v", base.Shots)
	}
	rules := DecodeMapRules(loader, 0x100)
	if len(rules) != 26 {
		t.Fatalf("original tile triggers = %d; want 26", len(rules))
	}
	for _, rule := range rules {
		if rule.Definition.Width%16 != 0 || rule.Definition.PlaneStride*5 != rule.Definition.FrameStride {
			t.Fatalf("invalid original object descriptor: %+v", rule.Definition)
		}
	}
}

func TestMovementTiltAndBounds(t *testing.T) {
	e := newFixture(t)
	e.Players[0].X, e.Players[0].Y = 0, 2
	e.Tick([2]Input{{X: -1, Y: -1}})
	if e.Players[0].X != 0 || e.Players[0].Y != 2 {
		t.Fatalf("ship crossed its original upper-left limits: %+v", e.Players[0])
	}
	for range 12 {
		e.Tick([2]Input{{X: 1, Y: 1}})
	}
	// The second PAL field reuses the previous field's latched joystick input.
	if e.Players[0].X != 22 || e.Players[0].Y != 24 || e.Players[0].Tilt != 6 {
		t.Fatalf("diagonal speed or four-frame tilt cadence changed: %+v", e.Players[0])
	}
	for range 12 {
		e.Tick([2]Input{})
	}
	if e.Players[0].Tilt != 3 {
		t.Fatalf("ship did not return to its neutral graphic: %+v", e.Players[0])
	}
}

func TestPrimaryRepeatIsSixteenGameUpdates(t *testing.T) {
	e := newFixture(t)
	var frames []int
	for range 70 {
		e.Tick([2]Input{{Fire: true}})
		for _, event := range e.Events {
			if event.Kind == "shot" {
				frames = append(frames, e.Frame)
			}
		}
	}
	if len(frames) != 3 || frames[0] != 1 || frames[1] != 33 || frames[2] != 65 {
		t.Fatalf("primary-fire PAL frames = %v; want [1 33 65]", frames)
	}
}

func TestDeathRespawnAndGameOver(t *testing.T) {
	e := newFixture(t)
	e.Players[0].Level, e.Players[0].Nova = 5, 0
	e.HitPlayer(0)
	for range 70 {
		e.Tick([2]Input{})
	}
	p := e.Players[0]
	if p.Lives != 2 || p.Level != 2 || p.Nova != 3 || p.Invulnerable != 300 || p.Respawn != 145 {
		t.Fatalf("original respawn contract changed: %+v", p)
	}
	for range 300 {
		e.Tick([2]Input{})
	}
	e.Players[0].Lives = 1
	e.HitPlayer(0)
	for range 70 {
		e.Tick([2]Input{})
	}
	if e.Mode != GameOver || e.Players[0].Lives != 0 {
		t.Fatalf("final ship did not enter game over: %+v", e.Players[0])
	}
}

func TestPoolsAndIndependentPlayers(t *testing.T) {
	e := newFixture(t)
	e.Start(2)
	for index := range e.Players {
		e.Players[index].Respawn, e.Players[index].Invulnerable, e.Players[index].Y = 0, 0, 176
	}
	for range EnemyLimit + 2 {
		e.Spawn(Spawn{X: 100, Y: -80, Definition: Definition{Width: 16, Height: 16, Health: 2}})
	}
	if len(e.Enemies) != EnemyLimit {
		t.Fatalf("enemy pool grew to %d records", len(e.Enemies))
	}
	e.Tick([2]Input{{X: -1, Fire: true}, {X: 1, Fire: true}})
	if e.Players[0].X != 110 || e.Players[1].X != 162 || len(e.PlayerShots) != 4 {
		t.Fatalf("players did not retain independent control and shots: %+v", e.Players)
	}
}

func TestDecodedStageAndEndingGate(t *testing.T) {
	data := fixtureData()
	data.Stages = []Stage{{Height: 2}, {Height: 2, Boss: &Spawn{X: 100, Y: 20, Definition: Definition{Boss: true, Health: 0, Width: 16, Height: 16}}}}
	e, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	e.Start(1)
	e.Scroll = 0
	for range 8 {
		e.Tick([2]Input{})
	}
	if e.Stage != 1 || e.Mode != Playing || len(e.Enemies) != 1 {
		t.Fatalf("boss gate failed: stage=%d mode=%d enemies=%d", e.Stage, e.Mode, len(e.Enemies))
	}
	e.damageEnemy(0, 1, 0)
	e.Tick([2]Input{})
	if e.Mode != Ending {
		t.Fatal("destroyed final boss did not enter the ending")
	}
}

func TestSelectedStartingWeapon(t *testing.T) {
	data := fixtureData()
	data.Options.StartWeapon = 3
	e, err := New(data)
	if err != nil {
		t.Fatal(err)
	}
	e.Start(2)
	if e.Players[0].Weapon != 3 || e.Players[1].Weapon != 3 || e.Players[0].Level != 0 {
		t.Fatalf("selected starting weapon was not applied: %+v", e.Players)
	}
}

func TestShipsPassAboveGroundScenery(t *testing.T) {
	e := newFixture(t)
	p := e.Players[0]
	e.Spawn(Spawn{X: p.X, Y: p.Y, Definition: Definition{Width: 32, Height: 32, Health: 10, Ground: true}})
	e.Tick([2]Input{})
	if e.Players[0].Dying != 0 {
		t.Fatal("ground scenery incorrectly destroyed the flying ship")
	}
	e.Enemies[0].Definition.Ground = false
	for range 4 {
		e.Tick([2]Input{})
	}
	if e.Players[0].Dying == 0 {
		t.Fatal("a colliding flying object did not hit the ship")
	}
}

func TestOriginalRandomSpawnBytes(t *testing.T) {
	e := newFixture(t)
	e.Data.Random = []byte{255, 100, 200}
	definition := Definition{Width: 16, Height: 16, Health: 1}
	e.Spawn(Spawn{RandomMode: 6, Definition: definition})
	e.Spawn(Spawn{RandomMode: 1, Definition: definition})
	if e.Enemies[0].X != 191 || e.Enemies[1].X != 108 {
		t.Fatalf("original random schedule coordinates changed: %+v", e.Enemies)
	}
}

func TestOriginalEntryAnimation(t *testing.T) {
	e, err := New(fixtureData())
	if err != nil {
		t.Fatal(err)
	}
	e.Start(1)
	p := e.Players[0]
	if e.Scroll != 160 || p.Respawn != 130 || p.Invulnerable != 360 || p.Y != 208 {
		t.Fatalf("original level-entry state changed: scroll=%d player=%+v", e.Scroll, p)
	}
	for range 130 {
		e.Tick([2]Input{})
	}
	if e.Players[0].Y != 118 || e.Players[0].Respawn != 0 {
		t.Fatalf("original level-entry movement changed: %+v", e.Players[0])
	}
}

func TestOriginalFlyingPoolsAndClear(t *testing.T) {
	e := newFixture(t)
	for range 14 {
		e.Spawn(Spawn{Y: -60, Definition: Definition{FlyingPool: true, Health: 1}})
	}
	for range 20 {
		e.Spawn(Spawn{Y: -60, Definition: Definition{Ground: true, Health: 1}})
	}
	if len(e.Enemies) != 30 {
		t.Fatalf("original scenery and flying pools contain %d objects; want30", len(e.Enemies))
	}
	e.EnemyShots = []Bullet{{X: 1, Y: 1}}
	e.Spawn(Spawn{ClearObjects: true})
	if len(e.Enemies) != 18 || len(e.EnemyShots) != 1 {
		t.Fatalf("clear-flying marker affected unrelated pools: enemies=%d shots=%d", len(e.Enemies), len(e.EnemyShots))
	}
}

func TestVerifiedNativeFlyingMovement(t *testing.T) {
	e := newFixture(t)
	e.Spawn(Spawn{X: 80, Y: -20, Definition: Definition{FlyingPool: true, NativeKind: 12, Health: 10}})
	for range 4 {
		e.Tick([2]Input{})
	}
	if e.Enemies[0].Y != -17 {
		t.Fatalf("type12 scroll cadence changed: %+v", e.Enemies[0])
	}
	e.Spawn(Spawn{X: 20, Y: -20, Definition: Definition{FlyingPool: true, NativeKind: 1, Health: 10}})
	initialWorldX := 20 + e.CameraX
	for range 16 {
		e.Tick([2]Input{})
	}
	var tracking Enemy
	for _, enemy := range e.Enemies {
		if enemy.Definition.NativeKind == 1 {
			tracking = enemy
		}
	}
	if tracking.VX != 65536 || tracking.X+e.CameraX != initialWorldX+4 || tracking.Y != -4 {
		t.Fatalf("type1 acceleration changed: %+v", tracking)
	}
}
