package engine

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
)

// These hashes come from actual loader instructions in the development-only
// oracle. They cover defined routing and bonus seams, not whole-game equality.
func TestCampaignTransitionsAndBonusesAgainstOriginal68000(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if os.IsNotExist(err) {
		t.Skip("extract the original ADF for campaign comparisons")
	}
	if err != nil {
		t.Fatal(err)
	}
	gates, err := DecodeSurfaceGates(loader, 0x100)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("campaign_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var refs []struct {
		Name                      string
		Phase, Mask, Score, Bonus int
		Ticks                     int
		Hash                      string
	}
	if err := json.Unmarshal(encoded, &refs); err != nil {
		t.Fatal(err)
	}
	appendInt := func(state []byte, values ...int) []byte {
		for _, value := range values {
			state = binary.BigEndian.AppendUint32(state, uint32(value))
		}
		return state
	}
	for _, ref := range refs {
		t.Run(fmt.Sprintf("%s_phase%d_mask%d_score%d_bonus%d", ref.Name, ref.Phase, ref.Mask, ref.Score, ref.Bonus), func(t *testing.T) {
			e := &Engine{Data: &Data{Loader: loader, LoaderBase: 0x100, Stages: make([]Stage, 4), PlayerSpawnX: [2]int{112, 160}}, Campaign: NewCampaign(gates), CameraX: 48}
			for index, x := range []int{112, 160} {
				e.Players[index] = Player{Active: true, X: x, Y: 80, Tilt: 3, Lives: 3, Weapon: 2, Level: 3, Nova: 4, Score: ref.Score, WreckBonus: ref.Bonus}
			}
			var trace []byte
			if ref.Name == "prefill-entry" || ref.Name == "prefill-return" {
				resident, err := os.ReadFile("../../assets/unpacked/loddat.bin")
				if err != nil {
					t.Fatal(err)
				}
				e.Data.Random = resident[0x7400:0x7500]
				e.Data.MapRules = DecodeMapRules(loader, 0x100)
				e.Options = DefaultOptions()
				stage := ref.Phase
				if ref.Name == "prefill-return" {
					stage = 0
				}
				bank, err := os.ReadFile("../../assets/unpacked/" + []string{"lods0t", "lodst1", "lodst2", "lodst3"}[stage] + ".bin")
				if err != nil {
					t.Fatal(err)
				}
				mapOffset := 0x44000 - []int{0x44000, 0x2e89a, 0x2e4c0, 0x2e840}[stage]
				terrain := Stage{Mode: stage, Width: 24, Height: 8192, TileBank: bank[mapOffset+24576:]}
				for index := 0; index < 24576; index += 2 {
					terrain.Tiles = append(terrain.Tiles, binary.BigEndian.Uint16(bank[mapOffset+index:]))
				}
				e.Data.Stages[stage] = terrain
				scroll := 256
				if stage == 0 {
					scroll = gates[ref.Phase-1].ReturnScroll
					e.Campaign.ClearedMask = 1 << uint(ref.Phase)
				}
				e.SelectStage(stage, scroll)
				var objects [][]byte
				for _, enemy := range e.Enemies {
					if !enemy.Definition.Ground || enemy.Definition.FlyingPool {
						continue
					}
					s := enemy.ground
					object := []byte{enemy.Definition.Kind}
					object = binary.BigEndian.AppendUint16(object, uint16(enemy.X))
					object = binary.BigEndian.AppendUint16(object, uint16(enemy.Y))
					object = append(object, s.frame, s.flags&30, s.param, s.armor, s.secondary, s.flash, s.counterA, s.counterB, s.fire, s.burn)
					objects = append(objects, object)
				}
				slices.SortFunc(objects, bytes.Compare)
				trace = appendInt(trace, e.Scroll, e.CameraX, e.randomCursor&255, len(objects))
				for _, object := range objects {
					trace = append(trace, object...)
				}
			} else if ref.Name == "map-end" {
				e.Campaign.ActiveCave = ref.Phase
				e.Scroll = 8192
				for tick := 0; tick < ref.Ticks; tick++ {
					ready := true
					if tick == 0 {
						e.Scroll++
					}
					if ref.Phase == 0 {
						e.wrapSurface()
					} else {
						ready = e.Campaign.CaveReturnReady()
					}
					transition := 0
					if ref.Phase != 0 && ready {
						transition = 255
					}
					trace = appendInt(trace, e.Scroll, e.Campaign.EndHold, transition)
				}
			} else if ref.Name == "transition" {
				e.Campaign.ClearedMask = uint8(ref.Mask)
				phase := max(1, ref.Phase)
				entry := gates[phase-1].Progress + 80
				if _, err := e.Campaign.EnterCave(phase, entry); err != nil {
					t.Fatal(err)
				}
				stage, scroll := phase, 256
				if ref.Phase != 0 {
					scroll, err = e.Campaign.ExitCave()
					if err != nil {
						t.Fatal(err)
					}
					stage = 0
				}
				e.SelectStage(stage, scroll)
				trace = appendInt(trace, e.Scroll, e.CameraX, int(e.Campaign.ClearedMask))
				for _, p := range e.Players {
					trace = appendInt(trace, p.X, p.Y, p.Tilt, p.Invulnerable, p.Respawn, p.Lives, p.Weapon, p.Level, p.Nova, p.Score)
				}
			} else {
				if ref.Mask == 2 {
					e.Players[1].Active, e.Players[1].WreckBonus = false, 0
				}
				for clock := 0; clock < ref.Ticks; clock++ {
					if ref.Name == "wreck" {
						e.tickWreckBonus(clock)
					} else {
						e.ApplyOriginalEndingBonusTick()
					}
					for index, p := range e.Players {
						trace = appendInt(trace, p.Score, p.WreckBonus, e.Campaign.WreckAwarded[index])
					}
				}
			}
			if actual := fmt.Sprintf("%x", sha256.Sum256(trace)); actual != ref.Hash {
				if ref.Name == "prefill-entry" || ref.Name == "prefill-return" {
					original, _ := os.ReadFile(fmt.Sprintf("../../.cache/campaign-traces/%s-%d.bin", ref.Name, ref.Phase))
					t.Logf("original prefill state%x; native%x", original, trace)
				}
				t.Fatalf("native campaign fingerprint%s differs from original%s", actual, ref.Hash)
			}
		})
	}
}

func TestOriginalCampaignPlayableReturnProgress(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if err != nil {
		t.Skip("original loader is extracted locally")
	}
	gates, err := DecodeSurfaceGates(loader, 0x100)
	if err != nil {
		t.Fatal(err)
	}
	for index, want := range []int{4000, 5408, 7792} {
		if gates[index].ReturnScroll != want {
			t.Fatalf("cave%d playable return%d, want%d", index+1, gates[index].ReturnScroll, want)
		}
	}
}

func TestOriginalThirdCaveHoldsForThreeHundredTerrainUpdates(t *testing.T) {
	c := NewCampaign([]Gate{{Phase: 3}})
	if _, err := c.EnterCave(3, 7800); err != nil {
		t.Fatal(err)
	}
	if c.CaveReturnReady() || c.EndHold != 300 {
		t.Fatal("the map-end update did not start the original hold")
	}
	for update := 1; update <= 300; update++ {
		if ready := c.CaveReturnReady(); ready != (update == 300) {
			t.Fatalf("third-cave return at terrain update%d: ready%v", update, ready)
		}
	}
}

func TestOriginalSurfaceWrapRetainsShipsAndObjects(t *testing.T) {
	e := &Engine{Scroll: 8192, CameraX: 63, Enemies: []Enemy{{ID: 7}}, PlayerShots: []Bullet{{Slot: 0}}, EnemyShots: []Bullet{{Slot: 0}}}
	e.Players[0] = Player{X: 150, Y: 100, Active: true, Lives: 3, Invulnerable: 12}
	before := e.Players
	e.wrapSurface()
	if e.Scroll != 1 || e.CameraX != 63 || e.Players != before || len(e.Enemies) != 1 || len(e.PlayerShots) != 1 || len(e.EnemyShots) != 1 {
		t.Fatal("surface wrap reset state that the original preserves")
	}
}
