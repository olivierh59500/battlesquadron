package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestFormationRewardsAgainstOriginal68000(t *testing.T) {
	encoded, err := os.ReadFile("reward_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		X, Y, ClockStart, Others, Ticks int
		Hash                            string
	}
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	for _, r := range references {
		dead := Enemy{X: r.X, Y: r.Y, Health: -1, Definition: Definition{NativeKind: 0, FlyingPool: true}}
		e := &Engine{Data: &Data{}, Explosions: []Explosion{enemyExplosion(dead)}}
		if r.Others != 0 {
			e.Enemies = []Enemy{{Health: 1, Definition: Definition{NativeKind: 0, FlyingPool: true}}}
		}
		trajectory := []byte{}
		for tick := 0; tick < r.Ticks; tick++ {
			e.Frame = r.ClockStart + tick + 1
			e.advanceExplosions()
			row := make([]byte, 23)
			if len(e.Explosions) != 0 {
				explosion := e.Explosions[0]
				row[0] = 1
				row[2] = explosion.nativeCount
				binary.BigEndian.PutUint16(row[3:], uint16(explosion.X))
				binary.BigEndian.PutUint16(row[5:], uint16(explosion.Y))
				row[22] = byte(explosion.Frame)
			} else if len(e.Pickups) != 0 {
				pickup := e.Pickups[0]
				row[0] = 1
				row[1] = 5
				binary.BigEndian.PutUint16(row[3:], uint16(pickup.X))
				binary.BigEndian.PutUint16(row[5:], uint16(pickup.Y))
				binary.BigEndian.PutUint32(row[7:], uint32(pickup.native.x))
				binary.BigEndian.PutUint32(row[11:], uint32(pickup.native.y))
				binary.BigEndian.PutUint32(row[15:], uint32(pickup.native.vx))
				row[19] = pickup.native.direction
				row[20] = pickup.native.subtype
				row[21] = byte(pickup.Frame)
			}
			trajectory = append(trajectory, row...)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(trajectory))
		if hash != r.Hash {
			t.Fatalf("rewardX%d phase%d others%d native=%s original=%s", r.X, r.ClockStart, r.Others, hash, r.Hash)
		}
	}
}

func TestFlyingExplosionKeepsPhysicalCadence(t *testing.T) {
	enemy := Enemy{X: 100, Y: 20, Health: -1, Definition: Definition{NativeKind: 0, FlyingPool: true}}
	e := &Engine{Mode: Playing, Data: &Data{}, Frame: 1, Explosions: []Explosion{enemyExplosion(enemy)}}
	for frame := 2; frame <= 32; frame++ {
		e.Frame = frame
		e.advanceExplosions()
		if len(e.Pickups) != 0 {
			t.Fatalf("Nova reward appeared before the original32PAL boundary at%d", frame)
		}
	}
	e.Frame = 33
	e.advanceExplosions()
	if len(e.Explosions) != 0 || len(e.Pickups) != 1 || e.Pickups[0].Nova != 1 {
		t.Fatal("the final flying explosion did not become one original Nova capsule")
	}
}

func TestWeaponCarrierRewardsAgainstOriginal68000(t *testing.T) {
	resident, err := os.ReadFile("../../assets/unpacked/loddat.bin")
	if os.IsNotExist(err) {
		t.Skip("extract the original ADF to compare weapon-carrier rewards")
	}
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("carrier_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		X, Y, ClockStart, ClockStep, Random, Ticks int
		Hash                                       string
	}
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	if len(references) != 72 {
		t.Fatal("the original weapon-carrier case matrix is incomplete")
	}
	for _, r := range references {
		dead := Enemy{X: r.X, Y: r.Y, Health: -1, Definition: Definition{NativeKind: 6, FlyingPool: true}}
		e := &Engine{Data: &Data{Random: resident[0x7400:0x7500]}, randomCursor: r.Random, Explosions: []Explosion{enemyExplosion(dead)}}
		trajectory := []byte{}
		for tick := 0; tick < r.Ticks; tick++ {
			e.Frame = r.ClockStart + r.ClockStep*tick
			e.advanceExplosions()
			row := make([]byte, 23)
			row[0] = 1
			if len(e.Explosions) != 0 {
				explosion := e.Explosions[0]
				row[1] = 6
				row[2] = explosion.nativeCount
				binary.BigEndian.PutUint16(row[3:], uint16(explosion.X))
				binary.BigEndian.PutUint16(row[5:], uint16(explosion.Y))
				row[22] = byte(explosion.Frame)
			} else if len(e.Pickups) != 0 {
				pickup := e.Pickups[0]
				if pickup.Nova != 0 {
					t.Fatal("the original carrier grants a weapon, not Nova")
				}
				row[1] = 5
				binary.BigEndian.PutUint16(row[3:], uint16(pickup.X))
				binary.BigEndian.PutUint16(row[5:], uint16(pickup.Y))
				binary.BigEndian.PutUint32(row[7:], uint32(pickup.native.x))
				binary.BigEndian.PutUint32(row[11:], uint32(pickup.native.y))
				binary.BigEndian.PutUint32(row[15:], uint32(pickup.native.vx))
				row[19] = pickup.native.direction
				row[20] = pickup.native.subtype
				row[21] = byte(pickup.Frame)
			} else {
				t.Fatal("the original weapon carrier was discarded before its reward")
			}
			trajectory = append(trajectory, row...)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(trajectory))
		if hash != r.Hash {
			t.Fatalf("carrierX%d phase%d clockStep%d random%d native=%s original=%s", r.X, r.ClockStart, r.ClockStep, r.Random, hash, r.Hash)
		}
	}
}

func TestOriginalFlyingSlotsRemainHeldDuringRewards(t *testing.T) {
	e := &Engine{Options: DefaultOptions()}
	for slot := 0; slot < 12; slot++ {
		e.Explosions = append(e.Explosions, Explosion{Native: true, nativeCount: 8, PoolSlot: slot})
	}
	if e.Spawn(Spawn{Definition: Definition{FlyingPool: true, Health: 10}}) {
		t.Fatal("a flying slot was reused during its original death animation")
	}
	e.Explosions = e.Explosions[:11]
	if !e.Spawn(Spawn{Definition: Definition{FlyingPool: true, Health: 10}}) || e.Enemies[0].PoolSlot != 11 {
		t.Fatal("the highest actually free original flying slot was not selected")
	}
	e.Explosions = append(e.Explosions, enemyExplosion(Enemy{PoolSlot: 11, Definition: Definition{FlyingPool: true}}))
	if e.availableFlyingSlot() != -1 {
		t.Fatal("the source record represented by enemy and explosion was not counted once")
	}
	e.Spawn(Spawn{ClearObjects: true})
	e.Pickups = []Pickup{{PoolSlot: 11, SlotHeld: true}, {PoolSlot: 10, SlotHeld: true}}
	if e.availableFlyingSlot() != 9 {
		t.Fatal("original collectable capsules did not retain their source slots")
	}
	e.Explosions = []Explosion{{Native: true, PoolSlot: 9}, {Player: true}}
	e.EnemyShots = []Bullet{{X: 10, Y: 10}}
	e.Spawn(Spawn{ClearObjects: true})
	if len(e.Pickups) != 0 || len(e.Explosions) != 1 || !e.Explosions[0].Player || len(e.EnemyShots) != 1 {
		t.Fatal("the original flying-pool clear did not preserve player effects and hostile shots")
	}
}
