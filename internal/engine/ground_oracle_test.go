package engine

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// The reference fingerprints execute the original instructions in the isolated
// development-only oracle module. Runtime ground controllers use typed Go state.
func TestGroundControllersAgainstOriginal68000(t *testing.T) {
	loader, e := os.ReadFile("../../assets/unpacked/loader.bin")
	if e != nil {
		t.Skip("original loader is intentionally excluded from Git")
	}
	dat, e := os.ReadFile("../../assets/unpacked/loddat.bin")
	if e != nil {
		t.Skip("original resident data is local")
	}
	data, e := os.ReadFile("ground_oracle_test.json")
	if e != nil {
		t.Fatal(e)
	}
	var refs []struct {
		Kind, Mode, Template, X, Y, Ticks int
		Damage                            bool
		Hash, Trace                       string
	}
	if e = json.Unmarshal(data, &refs); e != nil {
		t.Fatal(e)
	}
	for _, r := range refs {
		t.Run(fmt.Sprintf("kind%d_mode%d_damage%v", r.Kind, r.Mode, r.Damage), func(t *testing.T) {
			definition, err := DecodeDefinition(loader, 0x100, uint32(r.Template))
			if err != nil {
				t.Fatal(err)
			}
			core := &Engine{Data: &Data{Loader: loader, LoaderBase: 0x100, Random: dat[0x7400:0x7500], Stages: []Stage{{Mode: r.Mode, Width: 24, Height: 8192}}}, Options: DefaultOptions(), Players: [2]Player{{Active: true, Lives: 3, X: 112, Y: 176}, {Active: true, Lives: 3, X: 160, Y: 176}}}
			enemy := Enemy{X: r.X, Y: r.Y, Frame: definition.Frame, Health: definition.Health, Definition: definition}
			core.initializeGround(&enemy)
			// The offline CPU fixture forces dispatch before game initialization;
			// its mutable trap global still has the loader's raw200, rather than
			// the captured medium game's100. Match that explicit input unchanged.
			enemy.ground.trapReset = loader[0x76cc-0x100]
			enemy.ground.fire = 1
			core.randomCursor = 0
			var trajectory []byte
			original, _ := os.ReadFile("../../.cache/ground-traces/" + r.Trace)
			for tick := 0; tick < r.Ticks; tick++ {
				if r.Damage && tick == 20 {
					enemy.ground.damage = 128
				}
				core.Enemies = []Enemy{enemy}
				groundStep(core, &enemy, tick*2)
				state := make([]byte, 14)
				binary.BigEndian.PutUint16(state, uint16(enemy.X))
				binary.BigEndian.PutUint16(state[2:], uint16(enemy.Y))
				s := enemy.ground
				copy(state[4:], []byte{s.frame, s.flags & 30, s.param, s.armor, s.secondary, s.flash, s.counterA, s.counterB, s.fire, s.burn})
				trajectory = append(trajectory, state...)
				if len(original) >= (tick+1)*14 {
					want := original[tick*14 : (tick+1)*14]
					if string(state) != string(want) {
						t.Fatalf("first differing NPC tick%d: original%x native%x", tick, want, state)
					}
				}
			}
			if actual := fmt.Sprintf("%x", sha256.Sum256(trajectory)); actual != r.Hash {
				t.Fatalf("nativegroundfingerprint%s original%s", actual, r.Hash)
			}
		})
	}
}
