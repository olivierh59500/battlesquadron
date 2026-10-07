package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestFlyingAllocationArmorAgainstOriginal68000(t *testing.T) {
	encoded, err := os.ReadFile("allocation_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var refs []struct {
		Kind, Players, TemplateHealth int
		Boundary                      bool
		Hash                          string
	}
	if err := json.Unmarshal(encoded, &refs); err != nil {
		t.Fatal(err)
	}
	if len(refs) != 36 {
		t.Fatal("all original descriptors and boundary cases are required")
	}
	for _, ref := range refs {
		t.Run(fmt.Sprintf("kind%d_players%d_health%d_boundary%t", ref.Kind, ref.Players, ref.TemplateHealth, ref.Boundary), func(t *testing.T) {
			armor := OriginalFlyingArmor(ref.TemplateHealth, ref.Players == 2)
			if actual := fmt.Sprintf("%x", sha256.Sum256([]byte{byte(armor)})); actual != ref.Hash {
				t.Fatalf("native allocation%s differs from original%s", actual, ref.Hash)
			}
			e := &Engine{Options: DefaultOptions()}
			for index := 0; index < ref.Players; index++ {
				e.Players[index].Active = true
			}
			if !e.Spawn(Spawn{Definition: Definition{FlyingPool: true, NativeKind: ref.Kind, Health: ref.TemplateHealth}}) {
				t.Fatal("original allocation failed")
			}
			if enemy := e.Enemies[0]; enemy.Health != armor || enemy.Definition.Health != ref.TemplateHealth {
				t.Fatalf("runtime armor or immutable template changed: %+v", enemy)
			}
			if ref.Kind == 2 || ref.Kind == 9 {
				e.initializeSpecial(&e.Enemies[0])
				if e.Enemies[0].special.hp != armor {
					t.Fatal("boss initialization replaced its allocated armor")
				}
			}
		})
	}
}

func TestGroundSpawnedFlyingArmorIsReducedOnce(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if err != nil {
		t.Skip("extract originals for ground-emitted flying allocation")
	}
	e := &Engine{Data: &Data{Loader: loader, LoaderBase: 0x100}, Options: DefaultOptions()}
	e.Players[0].Active = true
	if !e.queueGroundFlying(8, 100, 40, 0) {
		t.Fatal("original ground effect could not be queued")
	}
	queued := e.takeNativeGroundSpawns()
	if len(queued) != 1 {
		t.Fatal("ground effect allocation was lost")
	}
	full := queued[0].Definition.Health
	if !e.Spawn(queued[0]) || e.Enemies[0].Health != OriginalFlyingArmor(full, false) {
		t.Fatal("ground-emitted armor did not use the shared allocation rule exactly once")
	}
}
