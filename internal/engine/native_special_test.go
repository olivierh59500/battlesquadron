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

func originalBossFixture(t *testing.T) *Engine {
	t.Helper()
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if os.IsNotExist(err) {
		t.Skip("extract original ADF for boss regression fixtures")
	}
	if err != nil {
		t.Fatal(err)
	}
	resident, err := os.ReadFile("../../assets/unpacked/loddat.bin")
	if err != nil {
		t.Fatal(err)
	}
	e, err := New(&Data{Loader: loader, LoaderBase: 0x100, Random: resident[0x7400:0x7500], Stages: []Stage{{Mode: 0}}})
	if err != nil {
		t.Fatal(err)
	}
	e.Start(1)
	if err := e.StartFinalBattle(); err != nil {
		t.Fatal(err)
	}
	e.Scroll = 240
	for _, event := range e.Data.Stages[0].Events {
		e.Spawn(event)
	}
	// The independent boss harness injects template armor directly into its
	// records, bypassing allocation. Preserve that explicit post-allocation
	// input; one/two-player allocation has its own original-instruction cases.
	for index := range e.Enemies {
		e.Enemies[index].Health = e.Enemies[index].Definition.Health
	}
	return e
}

func TestFinalBattleStream(t *testing.T) {
	e := originalBossFixture(t)
	if len(e.Enemies) != 4 {
		t.Fatalf("original final-boss parts=%d;want4", len(e.Enemies))
	}
	positions := map[int][2]int{8: {90, -78}, 9: {10, -40}, 10: {186, -40}, 11: {90, -50}}
	for _, enemy := range e.Enemies {
		if [2]int{enemy.X, enemy.Y} != positions[enemy.PoolSlot] {
			t.Fatalf("wrong original part layout: %+v", enemy)
		}
	}
}

func TestFinalBossEntryAgainstOriginal68000(t *testing.T) {
	e := originalBossFixture(t)
	encoded, err := os.ReadFile("boss_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		Name  string
		Ticks int
		Hash  string
	}
	if err := json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	for _, reference := range references {
		e = originalBossFixture(t)
		var trajectory []byte
		for tick := 0; tick < reference.Ticks; tick++ {
			step := tick + 1
			if reference.Name == "final-damage" {
				for index, enemy := range e.Enemies {
					damage := 0
					if step == 40 && (enemy.PoolSlot == 9 || enemy.PoolSlot == 10) {
						damage = 90
					}
					if step == 80 && enemy.PoolSlot == 11 {
						damage = 90
					}
					if step == 120 && enemy.PoolSlot == 11 {
						damage = 128
					}
					if step >= 400 && step < 656 && enemy.PoolSlot == 11 {
						damage = 1
					}
					if damage != 0 {
						e.damageNativeSpecial(index, damage, 0)
					}
				}
			}
			e.Frame = 2 * (tick + 1)
			e.updateEnemies()
			state := make([]byte, 4*15+3)
			for slot := 0; slot < 4; slot++ {
				enemy := e.specialPart(slot+8, 2)
				if enemy == nil {
					t.Fatalf("part%d disappeared at tick%d", slot+8, tick)
				}
				destination := state[slot*15:]
				binary.BigEndian.PutUint32(destination, uint32(enemy.fixedX))
				binary.BigEndian.PutUint32(destination[4:], uint32(enemy.fixedY))
				destination[8], destination[9], destination[10] = byte(enemy.Frame), byte(enemy.special.hp), enemy.special.budget
				destination[11], destination[12], destination[13], destination[14] = byte(enemy.special.timer), byte(enemy.special.dying), byte(enemy.special.flash), 0
			}
			body := e.specialPart(8, 2)
			state[60], state[61], state[62] = byte(body.special.steerX), byte(body.special.steerY), byte(e.randomCursor)
			trajectory = append(trajectory, state...)
		}
		fingerprint := fmt.Sprintf("%x", sha256.Sum256(trajectory))
		if reference.Name == "final-damage" && e.Mode != Ending {
			t.Fatal("the verified final-body destruction did not trigger the ending")
		}
		if fingerprint != reference.Hash {
			if original, err := os.ReadFile("../../.cache/boss-traces/" + reference.Name + ".bin"); err == nil {
				for frame := 0; frame < len(original)/63 && frame < len(trajectory)/63; frame++ {
					if !bytes.Equal(original[frame*63:frame*63+63], trajectory[frame*63:frame*63+63]) {
						t.Logf("first mismatch tick%d source=%x native=%x", frame+1, original[frame*63:frame*63+63], trajectory[frame*63:frame*63+63])
						break
					}
				}
			}
			t.Errorf("%s native=%s original=%s", reference.Name, fingerprint, reference.Hash)
		}
	}
}
