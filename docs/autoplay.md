# Expert player validation

`internal/autoplay` supplies a deterministic validation player that acts through
the same two joysticks and fire/Nova buttons as a human. `Controller.Next` takes
an observed `engine.Engine` and returns `[2]engine.Input`. It never changes the
observed engine, options, ships, score, resources, stage, or campaign flags.

A campaign starts with `engine.Start(1)` or `engine.Start(2)` and the ordinary
three ships, basic weapon, three Nova charges, and original entry animation.
The controller uses the game's normal entry and respawn protection. It does
not enable the verification immunity option, upgrade the starting weapon, add
ships or Nova charges, teleport, call damage routines, or skip stage entrances.
Extra ships earned by the original score thresholds, replacement Nova capsules,
and the original death refill remain ordinary gameplay resources.

The player plans once per original input latch, every two PAL fields. It keeps
sixteen candidate routes through a forty-eight-field horizon, branches over the
nine legal joystick directions every four fields, and advances private engine
copies to evaluate hazards, attacks and rewards. These copies include decoded
future wave and terrain schedules, so newly arriving threats participate in
the forecast. Capsule targets use eighty-field forecasts of their original
oscillating movement to find reachable interception points.
If every retained route forecasts a death, nine bounded constant-direction
forecasts test a wider escape before committing a joystick action. Inactive
and exhausted ships supply zero joystick axes and buttons, including the
second-field latch path, so the physical input recording does not assert an
unused player's fire button.

The player aims through the original weak-point rectangles with primary-shot
offsets and travel time. Weapon carriers and formations receive extra priority
to obtain weapon upgrades and replacement Nova capsules. Ground scenery remains
an attack target, and destroyed turret wrecks are collected using the source's
canonical-position rectangle until its counter reaches 99. Each surface gate
receives advance positioning; every living player must satisfy the original
ten-update hold to enter its cave. Nova remains a finite resource and receives
a planning cost, while forecast survival and useful attacks can justify spending
it. Scarce charges receive a higher cost than surplus charges. Earlier contact
avoidance and a screen-edge cost preserve space to escape converging volleys.
Before level five, a weapon capsule and progress against its carrier outweigh
ordinary wreck rewards and the capped-Nova score bonus. Carrier damage receives
planning value because its guaranteed weapon capsule often appears beyond the
short forecast horizon; the game still supplies the actual damage, delayed
death animation, capsule and upgrade through ordinary play.
Fire is released during cooldown and pressed again when ready, an ordinary
button pulse that cancels the original automatic-repeat delay.

The search is a development verifier and is excluded from ordinary gameplay.
Its forecast workload must not be included in a desktop or Android frame-rate
measurement. The engine preserves its original fifty-field PAL logic cadence;
rendering performance is measured separately from gameplay simulation cadence.

Unit checks establish that planning leaves the observed game unchanged, that
moving capsules can be collected without altered resources, that both living
ships can satisfy a portal hold, and that repeated forecasts produce identical
input and state sequences. A complete native campaign demonstrates attainable
progression and exercises resource accounting, scenery destruction, portals,
bosses and ending logic. It does not establish equivalence with the Amiga.
The selected input trace must also be replayed against the original routines
or FS-UAE checkpoints, and pixel and audio comparisons remain separate checks.
The outstanding comparison boundaries are documented in [fidelity.md](fidelity.md).
The bounded same-input FS-UAE comparison, its 29 measured scalar fields and its
separate limitations are documented in [reference.md](reference.md).

## Final source-correct campaign

The final ordinary one-player run completed all three caves and the final arena
in 68301 gameplay PAL fields. Its recording contains 68401 fields, including
the final 100 ending-payout updates. It started with three ships, weapon zero
at level zero and three Nova charges; verification immunity remained disabled.
It finished with zero deaths, five ships earned through two original extra-life
awards, weapon three at level five, seven remaining Nova charges and a score of
2473920 after the ending payout.

The player collected 25 capsules and 155 turret wrecks, destroyed 526 ground
objects and 623 flying enemies, and spent two ordinary Nova charges. All three
cave-clear bits were set (`0x0e`), and the final encounter ended at its original
progress stop of 240. The maximum observed pools contained 21 actors, twelve
friendly projectiles and twelve hostile projectiles. A fresh input-only replay
matched every recorded checkpoint and the terminal state exactly.

The following checksum-only evidence identifies the local recording and final
state. Raw recordings, JSON reports and captures remain excluded from Git.

| Evidence | SHA-256 |
| --- | --- |
| Original resident program | `8d47738b722e304d7a07810d0bab6785fb3446bab4ca604643c3945221139f10` |
| Expert input recording | `5c539330adb517ded9bc69a95676c555366afae8432c0dad9772b33709a16e54` |
| Terminal native state | `467af6b438b0b2a5681233d3490923d11e066ebf5140e9a26db4cb46e627223b` |

The source extra-life routine also matched 34145 sampled states and both awards
from this exact recording; six transition seams were explicitly excluded. See
[lives.md](lives.md). These proofs establish native progression, input-only
determinism and the named original routine. They do not establish a complete
matched-input original campaign, pixel parity or analog audio parity.

Run the ordinary expert campaign and record its exact input sequence:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/verify -mode expert -frames 200000 \
  -report .cache/validation/expert.json \
  -record .cache/validation/expert.bsinput
```

Replay that recording through a fresh ordinary engine without any planning:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/verify -mode replay \
  -input .cache/validation/expert.bsinput \
  -report .cache/validation/replay.json
```

The expert run also performs its own fresh-engine replay and checks checkpoint
and terminal state digests. Reports and recordings remain local, outside Git.
The explicitly named `inspect` mode changes starting state and is unsuitable
for honest progression claims.
