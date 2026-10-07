# Original sound playback

`internal/sound` is a native Go translation of the Ron Klaren driver recovered
from the supplied disk. It reads the original song commands, instrument records,
signed sample bytes and waveform tables; it contains no CPU emulator and never
executes the original program. The separate development oracle is outside the
application module.

The sound overlays are recreated by the ADF extractor and excluded from Git:

| Overlay | Original address | Native role | CIA timer latch |
| --- | --- | --- | --- |
| `lodgam.bin` | `$246F0` | Ten gameplay songs and sixteen sample effects | `$3100` |
| `lodmus.bin` | `$3D800` | Title music | `$2500` |
| `lodcom.bin` | `$3D800` | Common transition music | `$2600` |
| `lodspe.bin` | `$246F0` | Sample bank shared by the title overlay | — |

The original CIA clock is one fifth of the PAL Paula clock, 3,546,895 Hz. Timer
updates use the original latch plus one and therefore keep their actual tempo
independently of display updates. The decoder preserves byte/word wraparound,
pattern repeats, transposition, twelve-step arpeggios, portamento, signed-table
vibrato, four-stage envelopes, mutable original waveforms, sample attacks and
channel borrowing for sound effects. Duration-zero notes continue parsing in
the same update, matching the original branch back into the command parser.

`Player.Read` supplies little-endian signed 16-bit stereo PCM. Paula channels
0 and 3 feed the left output; channels 1 and 2 feed the right. The mixer retains
sample location/length until the DMA block ends, reads signed eight-bit original
samples, and follows sample timing with an integer clock remainder. It models
the digital Paula path. The Amiga's analog output/filter response has not yet
been matched against an independently captured waveform.

Construct a player using the extracted assets and pass it to Ebiten's audio
player. `UseMenu(true)` selects the original title music; `UseMenu(false)` selects
gameplay. `PlayTrack` takes the original one-based song number. `PlayEffect`
takes an original effect code: bits 4–5 select the channel and the low nibble
selects a sample descriptor. For example, loader routine `$3F7C` uses code 57 for
Nova, and `$34B6` uses code 29 for a destroyed airborne enemy. Missing or invalid
original data returns an error; no generated substitute sounds are used.

## Independent validation

The optional oracle executes only the recovered sound routines during
verification, with a cached instruction-accurate Go 68000 core. It fingerprints
channel state, Paula registers, DMA enable bits and all original waveform data,
with channel/global state omitted from the waveform comparison. It is never
linked into either the desktop or Android application.

```sh
cd tools/sound-oracle
go run .
go run . -controls -out ../../internal/sound/control_oracle_test.json
cd ../..
go test ./internal/sound
```

The committed regression file contains SHA-256 fingerprints only, with no
original samples or songs. The current suite compares 120 checkpoints across
all twelve songs, up to 4,096 original timer updates per song, including sample
effects 17, 28 and 57. Every compared state and waveform fingerprint matches.
Eleven additional original comparisons exercise music pause/resume, effects
mute/unmute and a sample effect played while the song is paused. Music disabling
freezes the original command stream; enabling it resumes the saved position.
The PCM test verifies nonzero original audio and identical output when the audio
consumer splits a PCM frame across arbitrary read boundaries. Tests requiring
original data skip when the excluded extracted assets are absent.

Addresses and command meanings were verified against the supplied depacked
binary, using local disassembly and the execution oracle. The public
[CrownParkComputing research](https://github.com/CrownParkComputing/BattleSquadron-Amiga)
helped locate the original routines. Its native C implementation was not copied
into this project.
