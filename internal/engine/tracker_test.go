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

func TestTrackingMovementAgainstOriginal68000(t *testing.T) {
	encoded, err := os.ReadFile("tracker_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		Kind, Stage, X, Y, Ticks int
		Hash                     string
	}
	if err := json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	for _, reference := range references {
		e := &Engine{Data: &Data{Stages: []Stage{{Mode: reference.Stage}}}, Options: DefaultOptions(), CameraX: 48,
			Players: [2]Player{{X: 112, Y: 176, Active: true, Lives: 3}, {X: 160, Y: 176, Active: true, Lives: 3}}}
		enemy := Enemy{X: reference.X, Y: reference.Y, fixedX: reference.X << 16, fixedY: reference.Y << 16,
			Health: 10, Definition: Definition{NativeKind: 8, FlyingPool: true, TrackingFrames: 200}}
		var trajectory []byte
		for tick := 0; tick < reference.Ticks; tick++ {
			e.Frame = 2 * (tick + 1)
			if !e.moveNativeTracker(&enemy) || enemy.Health < 0 {
				t.Fatalf("kind-eight stage%d (%d,%d) exited at frame%d before the original", reference.Stage, reference.X, reference.Y, tick)
			}
			state := make([]byte, 20)
			binary.BigEndian.PutUint32(state, uint32(enemy.fixedX))
			binary.BigEndian.PutUint32(state[4:], uint32(enemy.fixedY))
			binary.BigEndian.PutUint32(state[8:], uint32(enemy.VX))
			binary.BigEndian.PutUint32(state[12:], uint32(enemy.VY))
			state[16], state[17], state[18], state[19] = byte(enemy.Frame), byte(enemy.tracker.tracking), enemy.tracker.flags, byte(enemy.tracker.shotTimer)
			trajectory = append(trajectory, state...)
		}
		fingerprint := fmt.Sprintf("%x", sha256.Sum256(trajectory))
		if fingerprint != reference.Hash {
			if original, err := os.ReadFile(fmt.Sprintf("../../.cache/tracker-traces/tracker_%d_%d_%d.bin", reference.Stage, reference.X, reference.Y)); err == nil {
				for frame := 0; frame < len(original)/20 && frame < len(trajectory)/20; frame++ {
					if !bytes.Equal(original[frame*20:frame*20+20], trajectory[frame*20:frame*20+20]) {
						t.Logf("first mismatch frame%d source=%x native=%x", frame+1, original[frame*20:frame*20+20], trajectory[frame*20:frame*20+20])
						break
					}
				}
			}
			t.Errorf("kind-eight stage%d (%d,%d) native=%s original=%s", reference.Stage, reference.X, reference.Y, fingerprint, reference.Hash)
		}
	}
}
