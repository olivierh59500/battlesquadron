package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// These fingerprints come from the original $1CB0 routine, covering the entire
// emission, both owners, all eight ray boxes and every twelve-shot bank rewrite.
func TestNovaEmitterAgainstOriginal68000(t *testing.T) {
	loader, err := os.ReadFile("../../assets/unpacked/loader.bin")
	if os.IsNotExist(err) {
		t.Skip("extract the original ADF to compare Nova")
	}
	if err != nil {
		t.Fatal(err)
	}
	resident, err := os.ReadFile("../../assets/unpacked/loddat.bin")
	if err != nil {
		t.Fatal(err)
	}
	config, err := DecodeNova(loader, 0x100, resident)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("nova_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []struct {
		Owner, X, Y, ClockStart, Tick, Counter, Rays int
		ShotBank                                     bool
		Hash                                         string
	}
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	if len(references) != 1212 {
		t.Fatal("the complete original Nova reference is missing")
	}
	var e *Engine
	for _, r := range references {
		if r.Tick == 1 {
			e = &Engine{Data: &Data{Nova: config}, NovaFrames: 255, NovaOwner: r.Owner}
			e.Players[r.Owner] = Player{X: r.X, Y: r.Y}
		}
		e.Frame = r.ClockStart + r.Tick
		e.PlayerShots = nil
		e.updateNova()
		if e.NovaFrames != r.Counter || len(e.NovaRays) != r.Rays || (len(e.PlayerShots) != 0) != r.ShotBank {
			t.Fatalf("Nova owner%d tick%d native(counter%d,rays%d,shots%d) original(counter%d,rays%d,bank%v)", r.Owner, r.Tick, e.NovaFrames, len(e.NovaRays), len(e.PlayerShots), r.Counter, r.Rays, r.ShotBank)
		}
		state := []byte{byte(e.NovaFrames), byte(e.NovaFrames)}
		for _, ray := range e.NovaRays {
			row := make([]byte, 12)
			binary.BigEndian.PutUint16(row, uint16(ray.X))
			binary.BigEndian.PutUint16(row[2:], uint16(ray.Y))
			binary.BigEndian.PutUint16(row[4:], uint16(ray.X+16))
			binary.BigEndian.PutUint16(row[6:], uint16(ray.Y+16))
			row[8] = 16
			row[9] = ray.Graphic
			state = append(state, row...)
		}
		if r.ShotBank {
			if len(e.PlayerShots) != 12 {
				t.Fatal("Nova did not replace the complete twelve-shot bank")
			}
			for _, shot := range e.PlayerShots {
				row := make([]byte, 12)
				binary.BigEndian.PutUint16(row, uint16(shot.X-r.X))
				binary.BigEndian.PutUint16(row[2:], uint16(shot.Y-r.Y))
				binary.BigEndian.PutUint16(row[4:], uint16(shot.VX))
				binary.BigEndian.PutUint16(row[6:], uint16(shot.VY))
				row[8] = byte(shot.Height)
				row[9] = shot.Graphic
				row[10] = byte(shot.Delay)
				row[11] = byte(int8(shot.Damage))
				state = append(state, row...)
			}
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(state))
		if hash != r.Hash {
			t.Fatalf("Nova owner%d clock%d tick%d native=%s original=%s", r.Owner, r.ClockStart, r.Tick, hash, r.Hash)
		}
	}
}
