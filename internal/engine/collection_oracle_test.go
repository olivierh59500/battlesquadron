package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestCapsuleCollectionAgainstOriginal68000(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if os.IsNotExist(err) {
		t.Skip("extract the supplied ADF for original collection comparisons")
	}
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("collection_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var refs []struct {
		Owner, Weapon, Level, Nova, Subtype, Score, Cooldown, Repeat int
		Hash                                                         string
	}
	if err := json.Unmarshal(encoded, &refs); err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		t.Run(fmt.Sprintf("owner%d_weapon%d_level%d_nova%d_subtype%d", ref.Owner, ref.Weapon, ref.Level, ref.Nova, ref.Subtype), func(t *testing.T) {
			e := &Engine{Data: &Data{Loader: loader, LoaderBase: 0x100}}
			for index := range e.Players {
				e.Players[index] = Player{Weapon: 2, Level: 2, Nova: 5, Score: 12340, Cooldown: 3, Repeat: 4}
				e.PlayerShots = append(e.PlayerShots, Bullet{Player: index, Slot: 0}, Bullet{Player: index, Slot: 1})
			}
			e.Players[ref.Owner] = Player{Weapon: ref.Weapon, Level: ref.Level, Nova: ref.Nova, Score: ref.Score, Cooldown: ref.Cooldown, Repeat: ref.Repeat}
			pickup := Pickup{Weapon: ref.Subtype >> 1}
			if ref.Subtype == 10 {
				pickup.Nova = 1
			}
			e.collect(ref.Owner, pickup)
			var state []byte
			for index, p := range e.Players {
				shots := 0
				for _, shot := range e.PlayerShots {
					if shot.Player == index {
						shots++
					}
				}
				for _, value := range []int{p.Weapon, p.Level, p.Nova, p.Score, p.Cooldown, p.Repeat, shots} {
					state = binary.BigEndian.AppendUint32(state, uint32(value))
				}
			}
			if actual := fmt.Sprintf("%x", sha256.Sum256(state)); actual != ref.Hash {
				t.Fatalf("native collection fingerprint%s differs from original%s", actual, ref.Hash)
			}
		})
	}
}
