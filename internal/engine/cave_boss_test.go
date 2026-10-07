package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestCaveBossMovementAgainstOriginal68000(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if os.IsNotExist(err) {
		t.Skip("extract originals for cave-boss comparison")
	}
	if err != nil {
		t.Fatal(err)
	}
	resident, err := os.ReadFile("../../assets/unpacked/loddat.bin")
	if err != nil {
		t.Fatal(err)
	}
	overlays := map[string][]byte{}
	for _, name := range []string{"lods0f", "lodst1", "lodst2", "lodst3"} {
		overlays[name], err = os.ReadFile("../../assets/unpacked/" + name + ".bin")
		if err != nil {
			t.Fatal(err)
		}
	}
	stages, err := NativeSchedules(loader, 0x100, overlays)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("cave_boss_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		Mode, Ticks int
		Hash        string
		Damage      bool
	}
	if err := json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	for _, reference := range references {
		e, err := New(&Data{Loader: loader, LoaderBase: 0x100, Random: resident[0x7400:0x7500], Stages: []Stage{{Mode: reference.Mode}}})
		if err != nil {
			t.Fatal(err)
		}
		e.Start(2)
		e.Players = [2]Player{{X: 112, Y: 176, Active: true, Lives: 3}, {X: 160, Y: 176, Active: true, Lives: 3}}
		e.Scroll = 2601
		if reference.Mode == 2 {
			e.Scroll = 7701
		}
		e.Spawn(Spawn{Definition: Definition{NativeKind: 10, FlyingPool: true}})
		e.Spawn(Spawn{Definition: Definition{NativeKind: 10, FlyingPool: true}})
		for _, event := range stages[reference.Mode] {
			if event.Definition.NativeKind == 9 && event.Progress == e.Scroll {
				e.Spawn(event)
			}
		}
		var trajectory []byte
		for tick := 0; tick < reference.Ticks; tick++ {
			step := tick + 1
			if reference.Damage {
				for index, enemy := range e.Enemies {
					if enemy.Definition.NativeKind != 9 {
						continue
					}
					if enemy.PoolSlot == 9 && (step == 40 || step == 80 && reference.Mode == 1) {
						e.damageNativeSpecial(index, 128, 0)
					}
					if enemy.PoolSlot == 8 && (step == 120 || step == 160) {
						e.damageNativeSpecial(index, 128, 0)
					}
					if reference.Mode == 2 && enemy.PoolSlot == 8 && (step == 200 || step == 300) {
						e.damageNativeSpecial(index, 128, 0)
					}
				}
			}
			e.Frame = 2 * (tick + 1)
			e.updateEnemies()
			state := make([]byte, 45)
			for slot := 0; slot < 2; slot++ {
				enemy := e.specialPart(slot+8, 9)
				if enemy == nil {
					t.Fatalf("mode%d part%d removed tick%d", reference.Mode, slot, tick)
				}
				destination := state[slot*22:]
				binary.BigEndian.PutUint32(destination, uint32(enemy.fixedX))
				binary.BigEndian.PutUint32(destination[4:], uint32(enemy.fixedY))
				binary.BigEndian.PutUint32(destination[8:], uint32(enemy.VX))
				destination[12], destination[13], destination[16] = byte(enemy.Frame), byte(enemy.special.hp), byte(enemy.special.flash)
				if reference.Mode == 1 {
					destination[14] = byte(enemy.special.timer)
				} else {
					destination[14], destination[15] = byte(enemy.special.steerX), byte(enemy.special.steerY)
				}
				if enemy.special.dead {
					destination[17] = 128
				}
				if reference.Mode == 1 && slot == 0 {
					binary.BigEndian.PutUint32(destination[18:], uint32(enemy.special.path%400))
				}
			}
			state[44] = byte(e.randomCursor)
			trajectory = append(trajectory, state...)
		}
		fingerprint := fmt.Sprintf("%x", sha256.Sum256(trajectory))
		if fingerprint != reference.Hash {
			if source, err := os.ReadFile(fmt.Sprintf("../../.cache/boss-traces/cave_%d_%v.bin", reference.Mode, reference.Damage)); err == nil {
				for frame := 0; frame < len(source)/45 && frame < len(trajectory)/45; frame++ {
					if !bytes.Equal(source[frame*45:frame*45+45], trajectory[frame*45:frame*45+45]) {
						t.Logf("mode%d firstdiff tick%d source=%x native=%x", reference.Mode, frame+1, source[frame*45:frame*45+45], trajectory[frame*45:frame*45+45])
						break
					}
				}
			}
			t.Errorf("cave%d movement native=%s original=%s", reference.Mode, fingerprint, reference.Hash)
		}
		if reference.Damage {
			e.Frame = 2 * (reference.Ticks + 1)
			e.updateEnemies()
			if e.specialPart(8, 9) != nil || e.specialPart(9, 9) != nil {
				t.Fatal("the destroyed original cave-boss group did not release its records")
			}
		}
	}
}
