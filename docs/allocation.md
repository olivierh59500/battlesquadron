# Original flying-object allocation armor

The supplied loader's flying allocator at `$7544` copies template armor to
record byte `$18`. Its `$75B8..$75C8` branch then removes one quarter in a
one-player game. A two-player game retains the full byte. The source tests
operand `$F4B4(A5)`; its signed displacement addresses actual RAM `$74B4` when
A5 is `$8000`. This flag represents the enabled two-player configuration.

The arithmetic is byte-sized: increment the armor byte, shift that byte right
twice, then subtract it from the original armor. Consequently zero and one
remain unchanged, three becomes two, 31 becomes 23, and 127 becomes 95. `$FF`
remains `$FF` because its increment wraps to zero before the shift.

Native flying allocation now applies that rule once in `Engine.Spawn`.
Ground controllers that emit flying effects leave their templates unmodified
and use the same allocator, preventing a second reduction. Ground scenery has
its own separately translated allocation path.

Multipart boss initialization retains its allocated armor rather than copying
the full template armor again. Later controller assignments remain separate:
the final head's phase change explicitly reloads 127 at source `$93F6` and
receives no second allocation reduction. Immutable decoded templates retain
their original values throughout.

`allocation_oracle_test.json` contains 36 independent source fingerprints:
all fourteen original descriptor armor values under both player configurations,
plus explicit zero/one/three/`$FF` boundaries. The development oracle executes
actual `$7544` instructions through the armor branch ending at `$75CA`.
The application uses byte arithmetic in Go and imports no CPU emulator.

```sh
GOCACHE="$PWD/.cache/go-build" go -C tools/sound-oracle run . \
  -allocations -out ../../internal/engine/allocation_oracle_test.json
GOCACHE="$PWD/.cache/go-build" go test ./internal/engine \
  -run 'Allocation|GroundSpawnedFlying' -count=1
```

Existing adaptive-controller fixtures deliberately inject their post-allocation
state directly. The independent final-boss oracle likewise injects full template
armor into original records and bypasses allocation; its native fixture retains
that explicit input. These source-controller fingerprints remain unchanged.
The new allocator comparisons establish the player-count boundary separately.

The kind-six weapon carrier has template armor 31 and score ten. Its running
one-player armor is therefore 23, requiring damage greater than 23 to destroy it.
The earlier native allocation retained 31 and made those carriers harder to kill.
Its reward pipeline remains independently checked: six original death-animation
counts take roughly 22–24 physical fields before the record transforms into a
weapon capsule and immediately performs its first movement. A short planning
horizon can therefore miss the value of damaging a still-living carrier, even
when allocation and the reward pipeline are correct. Capsule movement and
inventory collection have their own original checks.

These cases establish armor initialization and its defined boss boundary, not
complete original damage/collision ordering or integrated whole-game parity.
