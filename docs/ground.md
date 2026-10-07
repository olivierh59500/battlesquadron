# Native ground and scenery controllers

`internal/engine/native_ground.go` translates the original eighteen-entry ground
pool into typed Go state. The runtime decodes each forty-eight-byte original
header, preserves byte-sized parameters/mailboxes/countdowns, and never imports
or executes a 68000 processor. NPC controllers run on the separately verified
25 Hz render cadence; their clock-bit selections use the 50 Hz original Clock.

Ground allocation consumes exactly one original random byte for
`(random & header[43]) + 1`. A single-player allocation also applies the original
quarter-armour reduction. New flying effects are queued during the stable ground
scan and allocated before the flying pass, so the original opening launcher's
new beam is processed in the same NPC update.

## Implemented state paths

- Types 0–5 preserve their distinct opening/closing, emergence, firing,
  vulnerability, secondary armour, flash and terminal sprite states.
- Type 1 preserves its nine-state opening sequence and emits exactly one
  original kind-seven growing ground beam.
- Type 32 preserves surface direction selection through signed division and the
  original turn table, its ten-count settling pause, cave-one triangular firing
  animation, cave-two paired kind-thirteen allocations and stationary cave-three
  live state. Destroyed wrecks retain their original death frames and point box.
- Type 33 has the original stationary live behavior. Type 34 preserves its
  eight-count alternating animation and special surface flash sequence.
- Types 37/38 preserve their one-shot random fuse and different muzzle offsets.
- Type 39 preserves the two original portal pictures and invulnerability;
  campaign routing owns the independently tested ten-update entrance detector.
- Type 40 preserves the periodic trap countdown, original frame lookup and
  frame-dependent firing point. Type 41 remains an invisible delayed trigger
  which can allocate an original kind-eight projectile.
- Shared types 32 and above consume the previous collision mailbox on the next
  NPC pass, preserve byte underflow and original flash/death frames, and keep
  wreck records instead of immediately replacing them with generic explosions.

`GroundCollisionBounds` uses the original horizontal offset at header word 20
and width at word 22. The collision y bounds retain the header's full sprite
height. This weak-point box is separate from the visible graphic rectangle.
The finished type-32 wreck uses its own inclusive point box, increments each
eligible ship's original bonus count to at most 99, and becomes invisible after
collection. The counter is preserved for the original later bonus payout.

The provided ADF uses the 208-line playfield. Its actual instruction constants
clear ordinary ground and beam records at y 208, clear the type-41 trigger at
y 158, and restrict type-5 cell firing below y 168. These are verified in the
local loader disassembly rather than inherited from the taller WHDLoad layout.

## Growing beams

Kind seven grows from one to thirty-two lines and probes all five original
terrain plane bytes. A byte equal to zero or `$FF` does not stop its leading
edge; any other value does. The core reads the immutable original tile bank
and current source map position. Surviving hits use the original flash frame;
a lethal hit selects the original eight-step enemy-explosion bank.
The renderer clips the original beam bitmap to the controller's current height.

The captured original game uses the medium ground preset: trap reload 100,
paired-launcher cycle 70, fan/beam cycles 50. The original presets select trap
reloads 75/100/125. Fixed surface-turret turn and aimed-shot intervals remain
10 and 50 respectively. These defaults were checked against saved original
RAM at `$76C8..$76CE`.

## Independent original-instruction verification

The separate `tools/sound-oracle` development module has a `-ground` mode. It
executes the supplied loader's actual instructions at `$5E9A`, stopping at the
bitmap seam `$6E44` or skipped-draw seam `$6F4C`. Audio and allocation call seams
are isolated from the controller state. The production executable does not
import this module or its processor dependency.

`ground_oracle_test.json` stores eighteen independent fingerprints covering
opening type 1; type 32 in all four modes; type 34 in surface/cave modes; type 37;
and type 40. Both normal and injected-damage cases run eighty NPC updates each,
1,440 updates in total. The native test compares position, sprite state, flags,
parameter, both armour bytes, flash, both counters, firing countdown and burn
state against the original trajectory/fingerprint.

The type-40 oracle deliberately starts directly from loader RAM before game
initialization. Its mutable trap reload therefore remains the raw value 200.
The native oracle fixture injects that same explicit state dependency, preserving
the original fingerprints; the running game uses the captured medium value 100.
This distinction prevents changing reference outputs to hide an input mismatch.

Additional tests cover delayed damage, wreck retention/claim/capping, collision
weak points, one-shot launcher allocation, turn settling, and individual-plane
beam probing and height/death behavior.

## Verification limits

The original-instruction fingerprints cover the named controller scenarios,
not every possible combination of shared globals, overlapping scenery or user
settings. Types 0/2/3/4/5 are translated from their instruction branches and have
behavior tests, but do not yet have the same complete original-CPU trajectories.

The beam probe uses original clean terrain bytes; the Amiga probe reads the
rendered ring, which can also contain another object's pixels. That composite
interaction still needs a renderer-aware comparison. Temporary impact events
use recovered original explosion art, while their full two-slot blit/palette
sequence is not independently fingerprinted. Ground controllers allocate the
original kind-thirteen events; that flying class's complete independent motion
is outside this ground verification boundary. Exact campaign fade/return
presentation and all projectile palette animation phases remain separate
verification work.
