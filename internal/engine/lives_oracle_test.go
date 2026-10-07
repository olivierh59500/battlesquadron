package engine

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// The fingerprints execute actual $1340 instructions in the separate oracle.
// Each native fixture maps its total ship count to the original spare byte;
// this distinction matters during ordinary entry versus cave return.
func TestOriginalExtraLivesAgainstOriginal68000(t *testing.T) {
	encoded, err := os.ReadFile("lives_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var refs []struct {
		Previous, Current, Spare, Clock int
		Hash                            string
	}
	if err := json.Unmarshal(encoded, &refs); err != nil {
		t.Fatal(err)
	}
	for _, ref := range refs {
		for _, phase := range []string{"active", "dying", "ordinary-entry", "cave-return"} {
			if phase == "ordinary-entry" && ref.Spare == 0 {
				// No ship can be entering from an empty ordinary-entry stock.
				continue
			}
			t.Run(fmt.Sprintf("%s_%d_to_%d_spare%d_clock%d", phase, ref.Previous, ref.Current, ref.Spare, ref.Clock), func(t *testing.T) {
				e := &Engine{Frame: ref.Clock}
				owner := ref.Clock >> 2 & 1
				keepsCurrentShip := phase != "ordinary-entry"
				for index := range e.Players {
					p := &e.Players[index]
					p.Active, p.Score, p.lastLifeScore = true, 42000, 42000
					if index == owner {
						p.Score, p.lastLifeScore = ref.Current, ref.Previous
					}
					p.Lives = ref.Spare
					if keepsCurrentShip {
						p.Lives++
					}
					if phase == "ordinary-entry" || phase == "cave-return" {
						p.Respawn = 40
					}
					p.entryKeepsShip = phase == "cave-return"
					if phase == "dying" {
						p.Dying = 30
					}
				}
				e.awardOriginalLife()
				awarded := byte(0)
				for _, event := range e.Events {
					if event.Kind == "extra-life" {
						awarded++
						if event.Player != owner {
							t.Fatal("extra life was assigned to an unsampled player")
						}
					}
				}
				state := []byte{awarded}
				for _, p := range e.Players {
					spare := p.Lives
					if keepsCurrentShip {
						spare--
					}
					state = append(state, byte(spare))
					state = append(state, fmt.Sprintf("%08d", p.lastLifeScore)[:4]...)
				}
				if actual := fmt.Sprintf("%x", sha256.Sum256(state)); actual != ref.Hash {
					t.Fatalf("native extra-life fingerprint%s differs from original%s", actual, ref.Hash)
				}
			})
		}
	}
}

func TestCaveReturnUsesOriginalSpareShipCap(t *testing.T) {
	e := newFixture(t)
	e.Data.Stages = []Stage{{}, {}}
	e.Players[0].Lives, e.Players[0].Score, e.Players[0].lastLifeScore = 4, 100000, 99999
	e.SelectStage(1, 256)
	if !e.Players[0].entryKeepsShip {
		t.Fatal("cave transition omitted the original preserved-ship flag")
	}
	e.Frame = 1
	e.awardOriginalLife()
	if e.Players[0].Lives != 5 {
		t.Fatal("cave-return entry incorrectly counted the active ship against the spare cap")
	}
}
