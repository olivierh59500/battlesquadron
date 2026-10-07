# Bounded original FS-UAE input comparison

The original game was run in the installed FS-UAE 3.1.66 with its supplied ADF
and existing A500 Kickstart. Two new short sessions used copies of a genuine
gameplay state under `.cache/reference/`; the earlier state files were untouched.
Raw states, input recordings, captures and decoded RAM remain excluded from Git.

## Exact original input recording

The installed emulator supports `--record=path`, which forces deterministic mode.
Its [version-specific recording implementation](https://github.com/FrodeSolheim/fs-uae/blob/v3.1.66/src/fs-uae/recording.c)
writes big-endian words containing frame markers, random/state checksums, raster
lines and input IDs/states. The session's held-right input was recorded as
event 61 at frame 27, raster line 127. Both compared checkpoints occur later,
while that input is still held; no further input transition occurs between them.

Restoring a USS file loads its `.fs-uae-recording` sidecar, moves to the end of
that sidecar and sets the emulator's frame count accordingly. Supplying an empty
sidecar beside a copied state successfully bootstraps a new exact input recording
with relative frame zero. Restoring a state does not directly play the future
of a separately supplied movie: it replaces the command-line recording with the
state sidecar. A complete boot-origin movie remains the straightforward path for
long deterministic replay. The existing isolated states have no such prefix.

The SDL helper initiates actions with wall-clock delays, but the resulting
FS-UAE record identifies their actual emulated frame and raster positions.
Those positions, state sidecars and stored cycle counters provide the provenance
used by the Go diagnostic. Parsing the recording's checksums does not prove that
a second emulator playback has independently validated them.

## Same-phase opening result

The second capture pair compares frames 34 to 70 while right remains held.
The original and native results match all 29 selected scalar fields: Clock,
terrain progress, camera and both players' canonical position, tilt, total ship
count, weapon, weapon level, Nova, score, invulnerability, entry/death and firing
timers.

| Original measurement | Before | After |
| --- | ---: | ---: |
| Emulator record frame | 34 | 70 |
| CPU PC | `$0CE0` | `$0CCE` |
| Original Clock | 6094 | 6130 |
| Terrain progress | 303 | 321 |
| Camera | 49 | 63 |
| Player-one canonical X | 80 | 152 |
| Player-one invulnerability | 74 | 38 |

The stored `CYCS` delta is 1309612032. In this independently measured PAL mode,
each field uses `512 * 227 * 313 = 36378112` stored units, giving exactly 36
physical fields and 36 original Clock/player updates. Both PCs lie in the
loader's post-player raster-wait block `$0CCE..$0CE6`; no gameplay-state writes
occur between its instruction positions.

The native diagnostic starts from the observed ship, scroll and camera scalars,
then runs 36 ordinary Go ticks with right held. It preserves the existing
original entry protection and original inventory. It does not reconstruct
the original enemy/projectile pools or claim a complete original initial state.
The window remains before the first flying wave and both ships remain protected.

## Why the first pair did not match terrain and camera

The first independent capture spans record frames 34 to 69: exactly 35 physical
PAL fields, but 34 original Clock/player updates. Its initial PC is `$0CE0`,
after both player passes. Its terminal PC is `$0BAE`, after the next terrain,
camera and NPC passes at `$0B2E/$0B34`, before that loop's first player pass.
The source therefore includes one additional terrain/camera update between the
mixed-phase captures. Its players and timers match the 34 native logical ticks;
its terrain and camera are each one pixel ahead of an atomic native checkpoint.
The separate same-phase capture resolves that comparison boundary without
changing runtime timing to compensate for a partial original loop.

CPU PC is at offset `$44` in this USS CPU payload. The
[upstream saved-state writer](https://github.com/FrodeSolheim/fs-uae/blob/v3.1.66/src/newcpu.cpp)
stores its eight-byte header, fifteen D0–D7/A0–A6 registers, and then PC. Offset
`$4C` contains USP. Older private inspection scripts read `$4C` as PC; the new
Go parser uses the verified layout.

## Reproducing the diagnostic

`cmd/reference` reads compressed USS chunks, cycle/PC provenance and exact
FS-UAE input records. Parser tests reject incomplete or discontinuous recordings
and validate signed event states, raster lines, compression and PC decoding.
Use locally captured checkpoints and their original sidecars:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/reference \
  -record .cache/reference/pal-same-phase/original.recording \
  -before '.cache/reference/pal-same-phase/states/Saved State 1.uss' \
  -after '.cache/reference/pal-same-phase/states/Saved State 2.uss' \
  -out .cache/validation/reference-same-phase.json
GOCACHE="$PWD/.cache/go-build" go test ./internal/reference
```

`-right-event` selects the observed right-action ID if another emulator version
uses a different mapping. Reports contain source/state/record hashes, exact
capture provenance and every matched or differing named scalar. Raw RAM is
omitted. The tool reports differences explicitly and does not silently adjust
terrain, camera or field count to make them match.

## Verification boundary

This is an integrated, recorded-input opening comparison for the named player,
timing and terrain/camera scalars. It is not a complete native/original game-state
comparison, a pixel comparison, a full original movie replay or evidence that a
hardcore native campaign reaches the same original campaign checkpoints.
Longer matched-input sessions covering enemies, buildings, pickups, cave routing,
bosses and ending still require aligned full state and presentation comparisons.
