package engine

import (
	"encoding/json"
	"os"
	"testing"
)

func TestAimedVelocityAgainstOriginal68000(t *testing.T) {
	encoded, err := os.ReadFile("aim_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct{ X, Y, Speed, VX, VY int }
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	if len(references) != 60 {
		t.Fatal("the original five-speed aim reference is incomplete")
	}
	for _, r := range references {
		vx, vy := AimedVelocity(r.X, r.Y, r.Speed)
		if vx != r.VX || vy != r.VY {
			t.Fatalf("originalaim(%d,%d,speed%d) native(%d,%d) original(%d,%d)", r.X, r.Y, r.Speed, vx, vy, r.VX, r.VY)
		}
	}
}
