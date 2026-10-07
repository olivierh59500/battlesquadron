package sound

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
)

type reference struct {
	Overlay     string
	Track, Tick int
	Hash        string
	DataHash    string
}

func originalAssets(t *testing.T) Assets {
	t.Helper()
	read := func(name string) []byte {
		b, e := os.ReadFile(filepath.Join("../../assets/unpacked", name+".bin"))
		if os.IsNotExist(e) {
			t.Skip("extract the original ADF to run sound oracle comparisons")
		}
		if e != nil {
			t.Fatal(e)
		}
		return b
	}
	return Assets{Game: read("lodgam"), Title: read("lodmus"), Common: read("lodcom"), Special: read("lodspe")}
}

// stateBytes uses the original data ABI only inside the comparison test. The
// production sequencer owns Go fields and contains no register interpreter.
func stateBytes(p *Player) []byte {
	flag := func(v bool) byte {
		if v {
			return 1
		}
		return 0
	}
	globalLength := 6
	if p.layout.gameplay {
		globalLength = 12
	}
	out := make([]byte, globalLength+1+248+44)
	out[0] = flag(p.idle)
	out[1] = p.master
	out[2] = flag(p.fade)
	out[3] = p.savedMaster
	if p.layout.gameplay {
		out[4] = flag(p.muted)
		out[5] = flag(p.effectsMuted)
		binary.BigEndian.PutUint16(out[6:], uint16(flag(p.restore)))
		binary.BigEndian.PutUint16(out[8:], p.pending)
		binary.BigEndian.PutUint16(out[10:], uint16(flag(p.busy)))
	} else {
		binary.BigEndian.PutUint16(out[4:], p.pending)
	}
	out[globalLength] = p.fadeCount
	for i, c := range p.channels {
		b := out[globalLength+1+i*62:]
		binary.BigEndian.PutUint32(b, 0xdff0a0+uint32(i)*16)
		binary.BigEndian.PutUint32(b[4:], c.instrument)
		binary.BigEndian.PutUint32(b[8:], c.song)
		binary.BigEndian.PutUint32(b[12:], c.list)
		binary.BigEndian.PutUint32(b[16:], c.pattern)
		copy(b[20:], c.arpeggio[:])
		for j, w := range []uint16{c.transpose, c.repeat, c.period, c.target, c.envelopeStage, c.vibratoPosition, c.arpeggioPosition, 1 << i} {
			binary.BigEndian.PutUint16(b[32+j*2:], w)
		}
		copy(b[48:], []byte{c.note, c.duration, c.slideNote, c.envelopeReload, c.envelopeCount, c.vibratoDelay, c.waveformCount, c.slideSpeed, 0, c.volume, flag(c.retrigger), flag(c.attack), c.attackCount, flag(c.muted)})
	}
	for i, v := range p.voices {
		b := out[globalLength+1+248+i*11:]
		binary.BigEndian.PutUint32(b, v.pointer)
		binary.BigEndian.PutUint16(b[4:], v.length)
		binary.BigEndian.PutUint16(b[6:], v.period)
		binary.BigEndian.PutUint16(b[8:], v.volume)
		b[10] = flag(v.enabled)
	}
	return out
}

func TestNativeSequencerAgainstOriginal68000(t *testing.T) {
	assets := originalAssets(t)
	encoded, e := os.ReadFile("oracle_reference_test.json")
	if e != nil {
		t.Fatal(e)
	}
	var references []reference
	if e = json.Unmarshal(encoded, &references); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{"lodgam", "lodmus", "lodcom"} {
		tracks := 1
		if name == "lodgam" {
			tracks = 10
		}
		for track := 1; track <= tracks; track++ {
			t.Run(fmt.Sprintf("%s/song%d", name, track), func(t *testing.T) {
				p, e := NewWithAssets(assets, 48000)
				if e != nil {
					t.Fatal(e)
				}
				if name == "lodmus" {
					p.UseMenu(true)
				} else if name == "lodcom" {
					p.UseCommon()
				}
				if track != 1 {
					p.PlayTrack(uint16(track))
				}
				for tick := 0; tick <= 4096; tick++ {
					for _, r := range references {
						if r.Overlay == name && r.Track == track && r.Tick == tick {
							state := stateBytes(p)
							hash := fmt.Sprintf("%x", sha256.Sum256(state))
							if hash != r.Hash {
								expected, e := os.ReadFile(fmt.Sprintf("../../.cache/sound-oracle/dumps/%s-%d-%d.bin", name, track, tick))
								if e == nil {
									for j := range min(len(state), len(expected)) {
										if state[j] != expected[j] {
											t.Logf("first differing byte %d: native$%02x original$%02x", j, state[j], expected[j])
											break
										}
									}
								}
								t.Fatalf("tick%d state=%s; original=%s", tick, hash, r.Hash)
							}
							waveforms := append([]byte(nil), p.data...)
							clear(waveforms[p.layout.global-p.layout.base-2 : p.layout.global-p.layout.base+uint32(globalSize(p))])
							clear(waveforms[p.layout.channel-p.layout.base : p.layout.channel-p.layout.base+248])
							if p.layout.gameplay {
								clear(waveforms[0x24e34-p.layout.base : 0x24f34-p.layout.base])
							}
							dataHash := fmt.Sprintf("%x", sha256.Sum256(waveforms))
							if dataHash != r.DataHash {
								t.Fatalf("tick%d waveform data=%s; original=%s", tick, dataHash, r.DataHash)
							}
						}
					}
					if name == "lodgam" {
						switch tick {
						case 42:
							p.PlayEffect(17)
						case 160:
							p.PlayEffect(28)
						case 560:
							p.PlayEffect(57)
						}
					}
					p.tick()
					if p.err != nil {
						t.Fatalf("tick%d: %v", tick, p.err)
					}
				}
			})
		}
	}
}

func globalSize(p *Player) int {
	if p.layout.gameplay {
		return 12
	}
	return 6
}

func TestPCMOriginalSamplesAndFragmentedReads(t *testing.T) {
	assets := originalAssets(t)
	a, e := NewWithAssets(assets, 48000)
	if e != nil {
		t.Fatal(e)
	}
	b, e := NewWithAssets(assets, 48000)
	if e != nil {
		t.Fatal(e)
	}
	a.PlayEffect(57)
	b.PlayEffect(57)
	whole := make([]byte, 48000*4)
	if _, e = io.ReadFull(a, whole); e != nil {
		t.Fatal(e)
	}
	fragmented := make([]byte, len(whole))
	for at := 0; at < len(fragmented); {
		end := min(at+137, len(fragmented))
		if _, e = io.ReadFull(b, fragmented[at:end]); e != nil {
			t.Fatal(e)
		}
		at = end
	}
	if !bytes.Equal(whole, fragmented) {
		t.Fatal("PCM changes with the audio consumer's read boundaries")
	}
	if bytes.Equal(whole, make([]byte, len(whole))) {
		t.Fatal("the original note and Nova samples produced silence")
	}
}

func TestEachOriginalOverlayProducesPCM(t *testing.T) {
	assets := originalAssets(t)
	for _, name := range []string{"lodgam", "lodmus", "lodcom"} {
		t.Run(name, func(t *testing.T) {
			p, err := NewWithAssets(assets, 44100)
			if err != nil {
				t.Fatal(err)
			}
			if name == "lodmus" {
				p.UseMenu(true)
			} else if name == "lodcom" {
				p.UseCommon()
			}
			pcm := make([]byte, 44100*4*5)
			if _, err = io.ReadFull(p, pcm); err != nil {
				t.Fatal(err)
			}
			if bytes.Equal(pcm, make([]byte, len(pcm))) {
				t.Fatal("original overlay produced silence")
			}
		})
	}
}

func TestInvalidSoundInputs(t *testing.T) {
	if _, e := New(nil, 48000); e == nil {
		t.Fatal("missing original overlay was accepted")
	}
	if _, e := New(make([]byte, 38108), 1); e == nil {
		t.Fatal("invalid output rate was accepted")
	}
}

func TestSoundControlsAgainstOriginal68000(t *testing.T) {
	assets := originalAssets(t)
	p, err := NewWithAssets(assets, 48000)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := os.ReadFile("control_oracle_test.json")
	if err != nil {
		t.Fatal(err)
	}
	var references []reference
	if err = json.Unmarshal(encoded, &references); err != nil {
		t.Fatal(err)
	}
	for tick := 0; tick <= 4096; tick++ {
		for _, r := range references {
			if r.Tick == tick {
				hash := fmt.Sprintf("%x", sha256.Sum256(stateBytes(p)))
				if hash != r.Hash {
					t.Fatalf("soundcontrols tick%d native=%s original=%s", tick, hash, r.Hash)
				}
				waveforms := append([]byte(nil), p.data...)
				clear(waveforms[0x251f6-0x246f0 : 0x25204-0x246f0])
				clear(waveforms[0x252a4-0x246f0 : 0x2539c-0x246f0])
				clear(waveforms[0x24e34-0x246f0 : 0x24f34-0x246f0])
				hash = fmt.Sprintf("%x", sha256.Sum256(waveforms))
				if hash != r.DataHash {
					t.Fatalf("soundcontrol waveform tick%d native=%s original=%s", tick, hash, r.DataHash)
				}
			}
		}
		switch tick {
		case 42:
			p.PlayEffect(17)
		case 100:
			p.SetMusicEnabled(false)
		case 200:
			p.SetMusicEnabled(true)
		case 160:
			p.PlayEffect(28)
		case 300:
			p.SetEffectsEnabled(false)
		case 400:
			p.SetEffectsEnabled(true)
		case 320:
			p.PlayEffect(28)
		case 560:
			p.PlayEffect(57)
		}
		p.tick()
		if p.err != nil {
			t.Fatal(p.err)
		}
	}
}
