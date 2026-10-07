package autoplay

import (
	"reflect"
	"testing"

	"github.com/olivierh59500/battlesquadron/internal/engine"
)

func fixture(t *testing.T, players int) *engine.Engine {
	t.Helper()
	data := &engine.Data{Options: engine.DefaultOptions()}
	for weapon := range data.Weapons {
		for level := range data.Weapons[weapon] {
			data.Weapons[weapon][level] = engine.Weapon{Cooldown: 1, Width: 8, Height: 8, BankSize: 1,
				Slots: [12]int8{1, 0, -1, -1, -1, -1, -1, -1, -1, -1, -1, -1},
				Shots: []engine.Shot{{X: 12, Y: -9, VY: -8, Height: 8, Graphic: 0x40, Delay: 1, Damage: 1}}}
		}
	}
	e, err := engine.New(data)
	if err != nil {
		t.Fatal(err)
	}
	e.Start(players)
	for index := range players {
		e.Players[index].Respawn = 0
		e.Players[index].Invulnerable = 0
		e.Players[index].Y = 160
	}
	return e
}

func TestPlanningOnlyReturnsInputs(t *testing.T) {
	e := fixture(t, 1)
	e.Pickups = []engine.Pickup{{X: 140, Y: 145, Weapon: 2}}
	e.Spawn(engine.Spawn{X: 180, Y: 70, VY: 2, Definition: engine.Definition{Width: 32, Height: 32, Health: 8, Score: 100, FireDelay: 8}})
	before := e.Clone()
	input := New().Next(e)
	if !reflect.DeepEqual(e, before) {
		t.Fatal("planning changed the observed game instead of a private forecast")
	}
	for _, player := range input {
		if player.X < -1 || player.X > 1 || player.Y < -1 || player.Y > 1 {
			t.Fatalf("planner returned a non-joystick direction: %+v", player)
		}
	}
}

func TestInactiveJoystickNeverAssertsButtons(t *testing.T) {
	e := fixture(t, 1)
	c := New()
	for range 8 {
		inputs := c.Next(e)
		if inputs[1] != (engine.Input{}) {
			t.Fatalf("inactive original joystick asserted an input at field %d: %+v", e.Frame, inputs[1])
		}
		e.Tick(inputs)
	}
	// A ship disabled after the first field must also stop supplying the
	// preceding button latch on the physical joystick.
	e = fixture(t, 2)
	c = New()
	inputs := c.Next(e)
	if !inputs[1].Fire {
		t.Fatal("active second ship did not produce an ordinary ready fire press")
	}
	e.Tick(inputs)
	e.Players[1].Active = false
	if inputs = c.Next(e); inputs[1] != (engine.Input{}) {
		t.Fatalf("disabled ship reused its prior joystick latch: %+v", inputs[1])
	}
}

func TestMovingCapsuleIsCollectedWithOrdinaryInputs(t *testing.T) {
	e := fixture(t, 1)
	e.Pickups = []engine.Pickup{{X: 140, Y: 140, Weapon: 2}}
	c := New()
	for range 160 {
		e.Tick(c.Next(e))
	}
	if e.Players[0].Level != 1 || len(e.Pickups) != 0 {
		t.Fatalf("moving capsule was missed: player=%+v pickups=%+v", e.Players[0], e.Pickups)
	}
	if e.Players[0].Lives != 3 || e.Players[0].Nova != 3 || e.Options.Invulnerable {
		t.Fatalf("collection used altered resources or immunity: %+v", e.Players[0])
	}
}

func TestCarrierDestructionProducesAnEarnedWeaponUpgrade(t *testing.T) {
	e := fixture(t, 1)
	e.Data.Random = []byte{0, 2, 4, 6}
	e.Spawn(engine.Spawn{X: 124, Y: 40,
		Definition: engine.Definition{FlyingPool: true, Kind: 6, NativeKind: 6, Width: 32, Height: 48, Health: 31, Score: 10}})
	// A nearby scenery score competes with the carrier's delayed upgrade.
	e.Spawn(engine.Spawn{X: 12, Y: 120,
		Definition: engine.Definition{Ground: true, Width: 32, Height: 32, Health: 0, Score: 5000}})
	c := New()
	exploded, capsule, collected := false, false, false
	for range 480 {
		e.Tick(c.Next(e))
		for _, explosion := range e.Explosions {
			exploded = exploded || explosion.WeaponCarrier
		}
		for _, pickup := range e.Pickups {
			capsule = capsule || pickup.Nova == 0 && pickup.SlotHeld
		}
		for _, event := range e.Events {
			collected = collected || event.Kind == "pickup" && event.Value == 7
		}
		if e.Players[0].Level == 1 {
			break
		}
	}
	if !exploded || !capsule || !collected || e.Players[0].Level != 1 {
		t.Fatalf("carrier did not yield an earned upgrade: explosion=%t capsule=%t collected=%t player=%+v", exploded, capsule, collected, e.Players[0])
	}
	if e.Players[0].Lives != 3 || e.Options.Invulnerable {
		t.Fatalf("carrier validation changed stock or enabled immunity: %+v", e.Players[0])
	}
}

func TestIncomingContactIsAvoidedWithoutNovaOrImmunity(t *testing.T) {
	e := fixture(t, 1)
	e.Players[0].Nova = 0
	e.Spawn(engine.Spawn{X: e.Players[0].X, Y: 130, VY: 2,
		Definition: engine.Definition{Width: 32, Height: 32, Health: 100, Score: 100}})
	c := New()
	for range 80 {
		e.Tick(c.Next(e))
		if e.Players[0].Dying != 0 || e.Players[0].Lives != 3 {
			t.Fatalf("expert flew into the incoming contact hazard: %+v", e.Players[0])
		}
	}
	if e.Players[0].Nova != 0 || e.Options.Invulnerable {
		t.Fatal("hazard avoidance altered resources or enabled immunity")
	}
}

func TestWideDescendingContactIsEscapedBeforeBottomTrap(t *testing.T) {
	e := fixture(t, 1)
	e.Players[0].X, e.Players[0].Y, e.Players[0].Nova = 108, 172, 0
	e.Spawn(engine.Spawn{X: 74, Y: 105,
		Definition: engine.Definition{FlyingPool: true, Kind: 12, NativeKind: 12, Width: 64, Height: 50, Health: 11, Score: 100}})
	c := New()
	for range 120 {
		e.Tick(c.Next(e))
		if e.Players[0].Dying != 0 || e.Players[0].Lives != 3 {
			t.Fatalf("expert delayed escape until trapped by the wide contact at field %d: %+v", e.Frame, e.Players[0])
		}
	}
}

func TestEdgeVolleyRetainsLateralEscapeRoom(t *testing.T) {
	e := fixture(t, 1)
	e.Players[0].X, e.Players[0].Y, e.Players[0].Nova = 256, 154, 0
	e.Spawn(engine.Spawn{X: 256, Y: 42,
		Definition: engine.Definition{Width: 48, Height: 48, Health: 100, Score: 100, FireDelay: 8}})
	c := New()
	for range 160 {
		e.Tick(c.Next(e))
		if e.Players[0].Dying != 0 || e.Players[0].Lives != 3 {
			t.Fatalf("expert remained trapped under a right-edge volley: %+v", e.Players[0])
		}
	}
	if e.Players[0].X > 232 {
		t.Fatalf("expert failed to restore lateral escape room: %+v", e.Players[0])
	}
}

func TestAllLivingShipsFollowOriginalPortalHold(t *testing.T) {
	e := fixture(t, 2)
	e.Data.Stages = []engine.Stage{{Width: 24, Height: 10000}, {Width: 24, Height: 10000}}
	e.Campaign = engine.NewCampaign([]engine.Gate{{Phase: 1, Progress: 200, WorldX: 192, Definition: engine.Definition{Kind: 39, Ground: true, Width: 32, Height: 32}}})
	c := New()
	for range 240 {
		e.Tick(c.Next(e))
		if e.Stage == 1 {
			break
		}
	}
	if e.Stage != 1 || e.Campaign.ActiveCave != 1 {
		t.Fatalf("two living ships failed to enter the original watch box: players=%+v campaign=%+v", e.Players, e.Campaign)
	}
	if e.Players[0].Lives != 3 || e.Players[1].Lives != 3 {
		t.Fatal("portal entry changed the starting ship resources")
	}
}

func TestForecastReplayIsDeterministic(t *testing.T) {
	first := fixture(t, 1)
	first.Pickups = []engine.Pickup{{X: 200, Y: 120, Nova: 1}}
	first.Spawn(engine.Spawn{X: 180, Y: 50, VY: 2, Definition: engine.Definition{Width: 32, Height: 32, Health: 8, Score: 100, FireDelay: 8}})
	second := first.Clone()
	a, b := New(), New()
	for field := range 200 {
		left, right := a.Next(first), b.Next(second)
		if left != right {
			t.Fatalf("input replay diverged at field %d: %v != %v", field, left, right)
		}
		first.Tick(left)
		second.Tick(right)
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("forecast replay changed game state at field %d", field)
		}
	}
}
