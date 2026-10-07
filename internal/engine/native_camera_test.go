package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCameraAgainstOriginal68000(t *testing.T) {
	encoded, err := os.ReadFile("camera_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		X                     [2]int
		Marker                [2]byte
		Before, Target, After int
	}
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	if len(references) != 120 {
		t.Fatal("the original camera case matrix is incomplete")
	}
	for _, r := range references {
		e := &Engine{CameraX: r.Before}
		for index, marker := range r.Marker {
			player := Player{X: r.X[index], Active: true, Lives: 3}
			if marker == 255 {
				player.Active = false
			}
			if marker == 150 {
				player.Respawn = 145
			}
			if marker == 100 {
				player.Dying = 70
			}
			e.Players[index] = player
		}
		if target := NewCameraTarget(e.Players); target != r.Target {
			t.Fatalf("camera X%v markers%v target%d original%d", r.X, r.Marker, target, r.Target)
		}
		e.updateCamera()
		if e.CameraX != r.After {
			t.Fatalf("camera step from%d native%d original%d", r.Before, e.CameraX, r.After)
		}
	}
}

func TestCameraKeepsOriginalWorldCoordinates(t *testing.T) {
	e := &Engine{CameraX: 48, Players: [2]Player{{X: 256, Active: true, Lives: 3}}, Enemies: []Enemy{{X: 100, fixedX: 100 << 16}}, EnemyShots: []Bullet{{X: 120, fixedX: 120 << 16}}, PlayerShots: []Bullet{{X: 130, fixedX: 130 << 16}}, Pickups: []Pickup{{X: 140}}, Explosions: []Explosion{{X: 150}, {X: 160, Player: true}}}
	e.updateCamera()
	if e.CameraX != 49 || e.Enemies[0].X != 99 || e.Enemies[0].fixedX != 99<<16 || e.EnemyShots[0].X != 119 || e.Explosions[0].X != 149 {
		t.Fatal("world objects moved relative to the original terrain")
	}
	if e.PlayerShots[0].X != 130 || e.Pickups[0].X != 140 || e.Explosions[1].X != 160 {
		t.Fatal("canonical ship effects or intrinsic capsules were shifted")
	}
}

func TestAbsoluteFlyingSpawnsUseOriginalWorldX(t *testing.T) {
	e := &Engine{CameraX: 48, Options: DefaultOptions()}
	definition := Definition{FlyingPool: true, Health: 10}
	if !e.Spawn(Spawn{X: 80, Y: -32, AbsoluteX: true, Definition: definition}) {
		t.Fatal("original absolute-position spawn was rejected")
	}
	if e.Enemies[0].X != 32 {
		t.Fatalf("absolute originalX1080 viewport=%d;expected32", e.Enemies[0].X)
	}
	if !e.Spawn(Spawn{X: 80, Y: -32, Definition: definition}) {
		t.Fatal("relative original spawn was rejected")
	}
	if e.Enemies[1].X != 80 {
		t.Fatal("relative original coordinates received a second camera offset")
	}
}
