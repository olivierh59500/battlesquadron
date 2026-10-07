package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestAdaptiveMovementAgainstOriginal68000(t *testing.T) {
	resident, err := os.ReadFile("../../assets/unpacked/loddat.bin")
	if os.IsNotExist(err) {
		t.Skip("extract the original ADF to compare adaptive controllers")
	}
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("controller_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		Kind, Stage, X, Y, Ticks int
		Hash                     string
		Damage                   bool
		Fires                    []struct{ Tick, X, Y, VX, VY int }
	}
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	for _, r := range references {
		e := &Engine{Data: &Data{Random: resident[0x7400:0x7500], Stages: []Stage{{Mode: r.Stage}}}, Options: DefaultOptions(), Players: [2]Player{{X: 112, Y: 176, Active: true, Lives: 3}, {X: 160, Y: 176, Active: true, Lives: 3}}}
		enemy := Enemy{X: r.X, Y: r.Y, fixedX: r.X << 16, fixedY: r.Y << 16, Health: 10, Definition: Definition{NativeKind: r.Kind, FlyingPool: true}}
		trajectory := []byte{}
		fireIndex := 0
		for tick := 0; tick < r.Ticks; tick++ {
			e.EnemyShots = nil
			if r.Damage && tick == 40 {
				e.nativeDamage(&enemy)
			}
			e.Frame = tick + 1
			if !e.moveNativeAdditional(&enemy) {
				t.Fatal("verified controller was not selected")
			}
			if enemy.Health < 0 {
				t.Fatalf("kind%d stage%d (%d,%d) removed at tick%d;originalruns%d", r.Kind, r.Stage, r.X, r.Y, tick, r.Ticks)
			}
			if fireIndex < len(r.Fires) && r.Fires[fireIndex].Tick == tick {
				expected := r.Fires[fireIndex]
				fireIndex++
				if len(e.EnemyShots) != 1 {
					t.Fatalf("kind%d tick%d produced%d shots; original1", r.Kind, tick, len(e.EnemyShots))
				}
				shot := e.EnemyShots[0]
				if shot.X != expected.X || shot.Y != expected.Y || shot.VX != expected.VX || shot.VY != expected.VY {
					t.Fatalf("kind%d tick%d shot(%d,%d,%d,%d) original(%d,%d,%d,%d)", r.Kind, tick, shot.X, shot.Y, shot.VX, shot.VY, expected.X, expected.Y, expected.VX, expected.VY)
				}
			} else if len(e.EnemyShots) != 0 {
				t.Fatalf("kind%d tick%d fired outside the original trigger", r.Kind, tick)
			}
			state := make([]byte, 21)
			binary.BigEndian.PutUint32(state, uint32(enemy.fixedX))
			binary.BigEndian.PutUint32(state[4:], uint32(enemy.fixedY))
			binary.BigEndian.PutUint32(state[8:], uint32(enemy.VX))
			binary.BigEndian.PutUint32(state[12:], uint32(enemy.VY))
			state[16] = byte(enemy.Frame)
			if r.Kind == 4 {
				state[17] = enemy.native.mode
			}
			state[18] = enemy.native.direction
			state[19] = enemy.native.opposite
			state[20] = byte(e.randomCursor)
			trajectory = append(trajectory, state...)
		}
		if fireIndex != len(r.Fires) {
			t.Fatal("native controller missed an original shot")
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(trajectory))
		if hash != r.Hash {
			original, err := os.ReadFile(fmt.Sprintf("../../.cache/sound-oracle/controller-dumps/%d-%d-%d-%d-%v.bin", r.Kind, r.Stage, r.X, r.Y, r.Damage))
			if err == nil {
				for at := range min(len(original), len(trajectory)) {
					if original[at] != trajectory[at] {
						t.Logf("first mismatch at tick%d field%d native%x original%x", at/21, at%21, trajectory[at], original[at])
						break
					}
				}
			}
			t.Errorf("kind%d stage%d (%d,%d) native=%s original=%s", r.Kind, r.Stage, r.X, r.Y, hash, r.Hash)
		}
	}
}
