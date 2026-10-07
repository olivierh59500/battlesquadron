# Original capsule collection

The native inventory update is compared with actual loader routine `$3600`,
loaded from the supplied ADF at base `$100`. The original movement and capsule
trajectories remain separately documented in [waves.md](waves.md).

A weapon capsule selects family `subtype >> 1` and increments the existing
weapon level up to five. Switching families preserves the accumulated level.
The source clears only the collecting player's twelve primary-shot records and
reloads its original weapon header through `$3688/$2ABA`. It preserves the
current cooldown word at player offset `$2E` and repeat byte at `$39`. Resetting
those timers would allow an earlier shot after collection.

A subtype-ten capsule adds one Nova up to eight. It preserves both primary-shot
banks and both firing timers. At level five, a weapon capsule still selects its
family and clears its shot bank, then awards the original score bonus. A Nova
capsule awards that bonus when eight charges are already present.

The bonus branch at `$3678` supplies endpoint `$4012` to the original decimal
adder. Its eight preceding data digits at `$400A..$4011` are `00010000`, giving
10000 points. This differs from the ending payout's adjacent 1000-point operand
at endpoint `$401A`. The eight-digit score discards overflow: collecting a
capped capsule at score 99999000 leaves score 9000. Reaching level five or eight
Nova charges for the first time awards no capped-inventory bonus.

`collection_oracle_test.json` holds 120 independent fingerprints. The separate
development oracle executes the source collection, weapon reload, decimal
adder and Nova HUD routines, skipping only its two external music calls.
Production desktop and Android applications do not import the CPU dependency.

The cases include both player owners, initial families zero and three, levels
zero/four/five, Nova counts seven/eight, all five capsule subtype choices and
score values covering ordinary addition, carry and overflow. Each fingerprint
compares both ships' family, level, Nova, score, cooldown, repeat and remaining
shot count. The unselected player's inventory and shot bank are included.

Reproduce the comparisons from the checkout root:

```sh
GOCACHE="$PWD/.cache/go-build" go -C tools/sound-oracle run . \
  -collection -out ../../internal/engine/collection_oracle_test.json
GOCACHE="$PWD/.cache/go-build" go test ./internal/engine \
  -run 'CapsuleCollection' -count=1
```

These checks establish the named inventory update with valid original capsule
subtypes and inventory bounds. They do not establish every collision rectangle,
collection order with simultaneous threats, presentation frame or sound timing.
An expert input recording still needs integrated comparison with the original
game before its entire campaign can be described as behaviorally identical.
