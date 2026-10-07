# Original wave schedules and native controllers

`NativeSchedules` decodes the supplied disk's twelve-byte flying-object records.
Each record contains a stage scroll threshold, signed X/Y coordinates, an object
kind and a movement-script address. It does not execute the original program.
The new-game table and flying-template table are found through their original
loader operands and validated against the recovered data.

| Stage | Schedule address | Original records | Source |
| --- | --- | ---: | --- |
| Surface | `$CF70` | 169 | Resident loader |
| First underground area | `$2E89A` | 139 | `lodst1.bin` prefix |
| Second underground area | `$D75E` | 161 | Resident loader |
| Third underground area | `$DEEC` | 205 | Resident loader |

All 674 records are decoded, including four flying-pool clear events. A clear
preserves scenery and hostile shots: the original clears only its twelve flying
records. The engine also preserves the separate eighteen-record scenery pool.
Stage-one movement pointers receive the same eight verified address relocations
as the cracked loader. The original ADF's SPIK unpacking does not use BOND's
stage-data reversal.

Original graphics and movement scripts share stage overlays. Script reads search
the resident loader plus the currently active bank: surface `LODS0F` at `$2E508`,
first underground `LODST1` at `$2E89A`, second `LODST2` at `$2E4C0`, third `LODST3`
at `$2E840`. `LODS0S` is a graphics bank; it does not supply a substitute schedule.

The first surface wave starts at scroll 330, 360 and 390, with three original
kind-one ships. At scroll 500, four directed ships enter at X 288, 328, 368 and
408, Y 20. Their first movement command lasts 40 updates, moves four pixels left
per update and displays original direction frame 12. These values come directly
from the original records and direction table, not a generated formation.

## Commands and controllers

Kind-zero formation commands contain duration, direction/speed, turn delta and
turn delay. The thirty-two signed X/Y vectors are read from the original `$CC58`
table. Movement uses the old direction; animation uses the direction after a
scheduled turn. `FF00` jumps to an original stream address, including loops;
`FF01` removes the ship. Kind-three and kind-thirteen scripts preserve the signed
duration and velocity commands, including the scrolling contribution. Their
variable-width mask animation still needs a separate pixel-level comparison.

Random spawn coordinates retain the source rules. Kind six uses
`(random & 127) + 64`; kinds one and eight use
`random + (nextRandom & 127) - 64`. Random bytes come from the original resident
bank at `$17400`; the source increments only its pointer's low byte.

Native adaptive movement translates the actual recovered handlers:

| Kind | Original handler | Native behavior |
| ---: | --- | --- |
| 1 | `$9538` | Horizontal homing and source vertical speed |
| 4 | `$8AA8` | Initial weaving, player-relative dive, source acceleration and sixteen-direction animation |
| 6 | `$886C` | Alternating random steering and three-quarter-pixel vertical movement |
| 8 | `$85C4` | Two-axis tracking, original 200-update tracking window and retreat |
| 12 | `$7C50` | One scroll pixel plus another pixel on odd updates |

Kinds four and six also preserve the original surviving-hit frame counters.
Kind four flashes through the second bank of sixteen frames according to bit
one of its eight-update counter; kind six uses the four original impact frames.
The source's packed decimal score words are decoded as decimal digits. For
example, `$5000` awards 5,000 points.

Only kind five is a collectable capsule. Kind three is an armed enemy, not a
pickup. Original collection at `$3600` grants one Nova for subtype ten, capped at
eight; other subtypes select `subtype >> 1` as the weapon family and increment
the current level, capped at five. Changing weapon family does not reset the
level. The source plays original song five for a Nova pickup and song six for a
weapon pickup. The last exploding kind-zero ship changes into a Nova capsule;
that delayed formation-reward path is a separate campaign integration boundary.

## Independent comparisons

The optional oracle executes original routines only during development. The
application contains native Go controllers and does not import a CPU emulator.
Committed reference files contain hashes, never original graphics or scripts.

```sh
cd tools/sound-oracle
go run . -waves -out ../../internal/engine/wave_oracle_test.json
go run . -controllers -out ../../internal/engine/controller_oracle_test.json
go run . -trackers -out ../../internal/engine/tracker_oracle_test.json
cd ../..
go test ./internal/engine
```

The directed-flight test compares all 56 actual kind-zero formations with the
original 68000 handler, through termination or 512 updates. Fixed-point X/Y and
displayed frames match. The adaptive-controller test compares 32 scenarios for
kinds four and six: two stage modes, four initial positions and runs with/without
a surviving hit. It compares positions, velocities, direction frames, internal
steering state and random cursor. Kind-eight tracking has eight additional
original comparisons through 240 updates, including its tracking-window expiry.

These comparisons establish the tested movement boundaries. They do not establish
complete campaign equivalence: multi-part boss state machines, all original firing
patterns, cave entry/return transitions, delayed formation rewards and the
variable-width mask rendering still require their own original-state comparisons.
