# Original extra-life sampling

The native extra-life rule is compared with the supplied loader's actual routine
at `$1340`, loaded at base `$100`. The separate development oracle executes that
routine; desktop and Android applications use the native Go translation only.

## Decimal comparisons and player selection

The original stores eight ASCII score digits at player offset `$6A..$71` and
keeps the first four sampled digits at `$72..$75`. Clock bit two selects the
player whose score is sampled on that call.

For scores below one million, the source grants a spare only when the
hundred-thousands digit changes from zero to one, two to three, or five to six.
Those correspond to 100000, 300000 and 600000 when scores advance normally.
A jump from 90000 to 310000 does not satisfy the source's previous-digit check
and earns no spare. A generic threshold-crossing rule would therefore differ.

For larger scores, the source compares the current millions digit with its
previous sample and grants a spare when that digit differs. This includes the
digit's wrap at ten million. Score wrap from 99999999 to zero does not grant one.
The score sample updates even when the spare count already prevents an award.

## Spare count versus native total ships

Original offset `$38` contains spare stock and is capped at four by the routine.
The native `Lives` value includes the live ship. While a ship is active or dying,
four source spares therefore correspond to five native total ships.

An initial or ordinary death entry counts its arriving ship inside the source
stock byte. At the end of that entry, `$5006` consumes one spare to make it the
live ship. Native total stock is consequently capped at four during that entry.

A cave transition differs: `$7448` sets byte `$29` to `$FF`. The entry-completion
branch clears that flag and suppresses consumption of another spare. The
existing ship remains separate from spare stock throughout its cave-return
animation, so the corresponding native total cap remains five. The native
`entryKeepsShip` flag preserves this distinction. Testing only `Respawn > 0`
previously imposed the wrong cap on cave returns.

## Independent reference cases

`lives_oracle_test.json` contains 80 original fingerprints: ten score pairs,
four initial spare counts and both selected player banks. The original routine
produces the awarded count, resulting spare byte and sampled score digits. Its
display routine runs; only the external sound call at `$24744` is skipped.

The native test checks those fingerprints in active, dying, ordinary-entry and
cave-return state mappings. Impossible ordinary entries with zero available
ships are excluded from those native embedding cases. Scalar original cases
still include spare zero, the cap at four, and an already-over-cap byte of five.

Recreate the independent references from the checkout root:

```sh
GOCACHE="$PWD/.cache/go-build" go -C tools/sound-oracle run . \
  -lives -out ../../internal/engine/lives_oracle_test.json
GOCACHE="$PWD/.cache/go-build" go test ./internal/engine \
  -run 'ExtraLife|CaveReturn' -count=1
```

## Samples from an actual expert input recording

The oracle can also replay ordinary native joystick inputs and pass their
encountered score, previous sampled digits, spare byte and Clock to `$1340`.
Each original output is compared immediately with its native result.

```sh
GOCACHE="$PWD/.cache/go-build" go -C tools/sound-oracle run . \
  -lives -life-players 1 \
  -life-input ../../.cache/validation/expert.bsinput \
  -out ../../.cache/validation/lives-replay.json
```

The final successful expert campaign contains 68401 recorded fields, including
100 ending-payout updates after its 68301 gameplay fields. Its 34145 original
routine samples and both extra-life awards matched. Six transition fields were
skipped explicitly because their later bonus payout changes the post-tick score,
hiding the preceding sampled value. The run started with three ships and
finished with five, with no deaths, earned weapon level five and all caves plus
the final encounter completed. The full progression scope and bounded FS-UAE
comparison are documented in [autoplay.md](autoplay.md) and
[reference.md](reference.md).

| Final comparison evidence | SHA-256 |
| --- | --- |
| Expert input recording | `5c539330adb517ded9bc69a95676c555366afae8432c0dad9772b33709a16e54` |
| Original resident program | `8d47738b722e304d7a07810d0bab6785fb3446bab4ca604643c3945221139f10` |
| Original routine output stream | `4400ec676a6638703d47e3e498528a3d8cd1c5dae0d681fe3ece0b8607c319c3` |
| Native routine output stream | `4400ec676a6638703d47e3e498528a3d8cd1c5dae0d681fe3ece0b8607c319c3` |

An earlier 62647-field attempt that ended in game over was also checked:
31319 routine samples and four awards matched, with five transition seams
excluded. That failed progression attempt used an earlier controller and is
historical diagnostic evidence rather than proof of the final completed run.
Input recordings and JSON reports remain excluded from Git; only the named
checksums and counts are documented here.

This comparison establishes the extra-life decisions on those actual native
states. The surrounding enemies, collisions and score generation still come
from the native game. It does not establish that an original full game would
reach those same states with the same joystick recording, or independently prove
the physical frame phase of every call to the routine. Longer matched-input
original sessions remain a separate requirement.
