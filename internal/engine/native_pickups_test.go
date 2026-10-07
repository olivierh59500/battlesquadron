package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestCapsuleMovementAgainstOriginal68000(t *testing.T) {
	encoded, err := os.ReadFile("pickup_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		X, Y, Subtype, Ticks int
		Hash                 string
	}
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	if len(references) != 20 {
		t.Fatal("the original capsule reference is incomplete")
	}
	for _, r := range references {
		e := &Engine{}
		pickup := Pickup{X: r.X, Y: r.Y, Weapon: r.Subtype >> 1}
		if r.Subtype == 10 {
			pickup.Nova = 1
		}
		trajectory := []byte{}
		for tick := 0; tick < r.Ticks; tick++ {
			e.Frame = tick + 1
			if !e.updateNativePickup(&pickup) {
				t.Fatalf("capsule at%d subtype%d exited before original at%d", r.X, r.Subtype, tick)
			}
			state := make([]byte, 15)
			binary.BigEndian.PutUint32(state, uint32(pickup.native.x))
			binary.BigEndian.PutUint32(state[4:], uint32(pickup.native.y))
			binary.BigEndian.PutUint32(state[8:], uint32(pickup.native.vx))
			state[12] = pickup.native.direction
			state[13] = pickup.native.subtype
			state[14] = byte(pickup.Frame)
			trajectory = append(trajectory, state...)
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(trajectory))
		if hash != r.Hash {
			t.Fatalf("capsuleX%d subtype%d native=%s original=%s", r.X, r.Subtype, hash, r.Hash)
		}
	}
}
