# Original campaign routing and bonus comparison

The native game returns from each cave to its original surface checkpoint. It
continues the remaining surface route, wraps that map, and selects the final
encounter only after all three cave-clear bits are present. These rules were
checked against instructions and state from the supplied ADF's loader at `$100`.
They do not use the different addresses from the WHDLoad research variant.

## Original transitions

The loader's `$71C6..$7218` phase checks select fixed surface map rows. The
transition then executes 256 terrain updates before giving control back to the
player. The surface entry position is not the return checkpoint.

| Completed cave | Reload row | Progress before prefill | Playable return progress |
| --- | ---: | ---: | ---: |
| First | `$EA` | 3744 | 4000 |
| Second | `$142` | 5152 | 5408 |
| Third | `$1D7` | 7536 | 7792 |

The first two reload rows precede their portal trigger by 112 pixels; the third
precedes it by 96 pixels. A cave entry similarly starts with progress zero and
fills 256 terrain pixels, leaving its first playable progress at 256.

The original ship transition routine at `$7448` restores each live, non-dying
ship to its original spawn X, Y 208, banking frame 3, entry counter 130 and
invulnerability counter 360. It clears the primary-projectile banks and firing
timers. The weapon, weapon level, remaining lives and Nova charges survive.
The 256 terrain updates keep ship timers and the original Clock frozen; the
camera converges to 48 while the ships have their source entry-state marker 150.

Native transitions now preserve that prefill's ground controllers and map
triggers. The original terrain and random-byte source reconstruct any resulting
scenery. Comparing the actual combined `$9BA4`, `$5E9A` and `$3000` routines
found a tile rule bug: after reading a tile with `MOVE.W (A1)+`, the source's
`$30(A1)` test selects the following row's following column. Checking the same
column suppressed an original building after the third cave. The native rule
now uses the original operand's position.

## Map end and final route

The original terrain pointer first falls below `$44000` when progress 8192
advances to 8193. Surface progress then wraps to 1. That wrap preserves ships,
the camera, active scenery, flying objects and shots.

An incomplete campaign restarts the ordinary surface schedule and offers its
uncleared cave entrances again. Once the clear mask is `$0E`, the wrap selects
the separate final schedule. That schedule clears flying objects at progress
198, allocates four final-boss parts at 200, and stops the arena at 240. Clearing
the third cave does not immediately start the final boss.

The first two caves leave their map immediately at the map-end condition. The
third uses `$9CBA` to wait another 300 terrain updates at progress 8193 before
starting its surface transition. Terrain runs at 25 Hz, so this hold spans 600
native PAL updates. Player movement and timers retain their own 50 Hz cadence.

## Original score increments

The eight unpacked decimal digits at `$731A..$7321` encode 1000 points per
collected building wreck. The original `$734A/$739E` payout starts at Clock 160
and processes one wreck per eligible player on each even Clock. It decrements
the player's packed-BCD remaining count and increments its awarded count.
Disabled ships lose their unclaimed count. Each count is capped at 99 when
collected.

The current instantaneous native transition settles those balances before
restoring the map. Its ending balances and discarded final decimal carry match
the source; its missing animated countdown is not temporally equivalent.

The ending has a separate 100-update score payout at `$0E10..$0E40`. Its original
digits at `$4012..$4019` also encode 1000 points per enabled ship per update,
giving 100000 points over the full payout. Both routines retain eight decimal
digits and discard overflow, as the original decimal adder at `$4044` does.
Disabled players receive neither bonus. Dying and respawning ships remain
eligible while their original life-state marker is below 175.

## Independent checks and reproduction

`campaign_oracle_test.json` contains 49 SHA-256 fingerprints produced by the
separate development module. It imports a 68000 processor solely to execute
original instructions during verification. Desktop and Android applications do
not import that module or execute guest instructions.

The independent cases cover:

- Six combined terrain/ground/map prefill scenarios: every cave entrance and
  every fixed surface return, including scenery fields and the random cursor.
- Eight source map-end cases, including completed and incomplete clear masks,
  the wrap to progress 1 and the third cave's complete 300-update hold.
- Seven fixed transition cases with preserved inventory and original ship entry
  states, across different incoming cave-clear masks.
- Twenty-four wreck-payout traces, including counts 0, 1, 10 and 99, disabled
  players, decimal carries and overflow, through 361 original Clock values.
- Four ending-payout traces through all 100 increments, including disabled
  players and decimal overflow.

Reproduce from the checkout root after extracting the ADF:

```sh
GOCACHE="$PWD/.cache/go-build" go -C tools/sound-oracle run . \
  -campaign -out ../../internal/engine/campaign_oracle_test.json
GOCACHE="$PWD/.cache/go-build" go test ./internal/engine \
  -run 'Campaign|SurfaceWrap|ThirdCave' -count=1
```

Only the fingerprints and tools belong in Git. Optional original prefill dumps
remain in `.cache/campaign-traces/` and are not application inputs.

## Remaining fidelity boundaries

These cases establish the named routing, terrain prefill, inventory and score
seams. They do not establish equality of a complete campaign played with the
same inputs on FS-UAE and the native game. The current SDL reference input
helper schedules wall-clock delays and cannot supply exact emulated frame
boundaries for a long matched-input replay.

The animated cave entrance/return text, bonus countdown, Copper fades and exact
duration of every transition are still absent from the native presentation.
The arena's original `LODFIN` load occurs at progress 240; the native renderer
selects that bank with the final route flag, so its earlier map presentation is
not an independently established pixel match. Extra-life decisions and spare
stock now have [their own original comparisons](lives.md); the complete physical
call phase and score-display updates still need integrated state comparisons.
A successful native expert-player run cannot resolve those original gaps.
