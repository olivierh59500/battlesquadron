# Performance and smooth presentation

The default renderer interpolates movement at the display refresh rate while
the native engine retains its original 50 Hz PAL fields and 25 Hz enemy,
camera and terrain updates. Both desktop and Android explicitly enable VSync;
`SetTPS(50)` only schedules simulation updates. It does not limit `Draw` to
50 frames per second.

## Movement between original fields

Six reusable snapshots preserve the last PAL fields. Presentation uses a
periodic clock and a common two-field visual delay, so 50 Hz ships and shots
and 25 Hz terrain and enemies can all move between their original positions.
The signed clock phase also accounts for early or late update dispatch on a
display frame. Clamping every early dispatch to zero would reintroduce visible
pauses even with a 60 FPS rendering loop.

Rendering never advances the engine, predicts damage, changes collision
coordinates or accelerates music. New actors, lost lives, teleports and stage
changes discard incompatible position history. Hostile shots interpolate in
world coordinates and then use the same smooth camera as their background.
Pausing presents stable native state. The original artwork, animation frames
and animation selection remain unchanged; interpolation smooths their positions.

The playfield is drawn into a reusable viewport at four samples per original
pixel, or the display's higher integer density, capped at eight. Source textures
use nearest-neighbor sampling. Ebitengine's `DrawFinalScreen` presents that
viewport directly at display resolution with a linear final sample. This
preserves intermediate positions on small Android displays; downsampling first
into a 320-pixel logical framebuffer would lose them. It also means the default
smooth presentation intentionally differs from original pixel and temporal
output. HUD text still comes from the extracted original bitmap font.

Use `go run . -original-cadence` or the same benchmark option to compare the
original discrete movement cadence and nearest-neighbor presentation. This
comparison mode does not establish full Amiga framebuffer parity.

## Measured work and frame pacing

The real desktop and Android loops are instrumented separately from headless
engine tests. Reports include observed Draw/Update rates, frame-interval and
CPU-work percentiles, allocation and GC totals, peak actor/projectile counts
and terrain-raster position changes. Warmup excludes initial asset decoding,
audio initialization and graphics uploads.

`draw_cpu` measures Go drawing submission, not GPU completion. Successive
Draw-start intervals include the graphics driver's pacing and stalls. The
Android compositor's separate presented timestamps were also inspected for
the game's GL surface. Its reported refresh interval was 16,666,666 ns and its
presented timestamps followed the same cadence.

Terrain motion counts compare rounded positions in the oversampled viewport.
They distinguish repeated 25 Hz state from intervening positions, but are not
a full composited-frame pixel comparison. The final scene has zero terrain
motion because its original backdrop stops scrolling.

The first desktop CPU profile identified rebuilding touch-circle paths as
one third of sampled CPU work. Caching existing touch geometry reduced draw
p95 from 2.368 ms to 0.120 ms, and process allocation from approximately 4,903
to 1,141 allocations per Draw in matching 165 Hz, audio-enabled runs. Keeping
sprite-frame and font subimages avoids repeated region-cache lookups and
wrapper recreation after Ebitengine's subimage cache expires.
Terrain startup now copies rows with Go's standard image drawing operation
instead of allocating an interface color per pixel; measured artwork/engine
startup changed from 298 ms to 167 ms in those runs.

At 60 Hz, the subsequent continuous-clock run measured 60.00 Draws/s,
49.99 PAL updates/s and 60.07 terrain-raster changes/s. Its original-cadence
comparison measured 60.00 Draws/s but only 25.03 terrain changes/s. This is
the relevant difference between a fast display loop and smooth movement.

The six-scene Android source snapshot was rebuilt and measured with the long forecast
workloads paused and screen recording disabled. Desktop audio, touch controls,
two players and maximum decoded weapon levels remained active. Android scenes
included their real touch overlay and audio mixer. Results from the local
2026-10-07/08 sessions were:

| Scene | Draws/s | PAL updates/s | Draw CPU p95 | Update CPU p95 | Interval p95 | Maximum interval | Terrain changes/s |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Desktop smooth surface | 164.58 | 49.97 | 0.044 ms | 0.050 ms | 6.306 ms | 13.035 ms | 99.88 |
| Android original-cadence surface | 59.50 | 49.98 | 0.186 ms | 0.102 ms | 18.968 ms | 56.717 ms | 24.94 |
| Android smooth surface | 59.90 | 49.99 | 0.214 ms | 0.055 ms | 17.693 ms | 46.587 ms | 60.00 |
| Android smooth cave one | 59.90 | 49.96 | 0.153 ms | 0.085 ms | 17.730 ms | 39.918 ms | 59.80 |
| Android smooth cave two | 60.00 | 49.99 | 0.163 ms | 0.054 ms | 17.388 ms | 18.288 ms | 60.10 |
| Android smooth cave three | 60.00 | 49.97 | 0.148 ms | 0.045 ms | 17.461 ms | 20.262 ms | 60.10 |
| Android smooth final encounter | 59.80 | 49.98 | 0.138 ms | 0.043 ms | 17.478 ms | 50.524 ms | 0.00 |

The desktop snapshot reached eighteen active player projectiles. Android cave
two reached fourteen enemies and all twelve allowed hostile projectiles;
cave three reached twelve enemies and twelve hostile projectiles. The emulator
held the 60 Hz cadence most of the time, with occasional longer presentation
intervals. GPU completion and OS scheduling were not separately timed, so those
stalls are not attributed to a single cause. They mean a strict minimum of 60 FPS
for every frame has not been demonstrated on Android. CPU gameplay and drawing
submission stay comfortably inside the 16.667 ms frame budget in these scenes.

These observations apply to the measured macOS ARM64 desktop and local
Android 35 ARM64 emulator with SwiftShader OpenGL ES 3. Short CPU timings,
a valid APK or average 60 FPS do not prove a strict per-frame deadline on
every physical Android device. Reports preserve slow-frame percentiles and
maximum intervals rather than hiding them behind an FPS average. The headless
emulator disables host audio output, although the game's PCM mixing and audio
player remain active during its measured runs.

The six-scene measurements above characterize ordinary movement and rendering
before the missing Nova bitmaps and source single-player health correction were
restored. The final APK's separate Nova check below exercises its visible
original particles; the earlier paired video remains a movement comparison.

On 2026-10-08, the final Android 35 ARM64/SwiftShader build was measured with
audio, touch controls, two active players and a real injected Nova. Across
9.55 measured seconds it produced 59.90 Draws/s, 49.95 PAL updates/s and 59.90
terrain-raster changes/s. Draw CPU p95 was 0.056 ms, Update CPU p95 0.022 ms,
frame-interval p95 17.403 ms and maximum interval 31.504 ms. The interval includes
the actual screenshot checkpoint. There were eighteen player projectiles and
nine effects at the measured peaks, with no garbage collection cycle.

Native snapshots show the Nova charge falling from three to two, twelve
projectiles and counter 230 at the visible capture, and continued progression
from field 1,150 to 1,608. The restored original spheres are visible in
`captures/android-nova-restored.png`; state and timing evidence is in
`captures/android-nova-performance.json`. This still does not establish a
strict minimum of 60 FPS for every Android frame.

A final macOS ARM64 check with the same restored artwork, two players, audio,
touch overlay and Nova measured 164.70 Draws/s over 11.99 seconds. Draw CPU p95
was 0.030 ms, Update CPU p95 0.022 ms, interval p95 6.146 ms and maximum interval
13.326 ms. Peaks included eighteen player projectiles and nine effects. The
single GC cycle paused for 0.105 ms, and artwork/engine startup took 166.11 ms.
The exact report is `captures/performance-desktop-final.json`; these final
observations cover the restored visible Nova rather than only its earlier logic.

## Reproduce

All outputs below stay in ignored `captures/`, `bin/` and `.cache/` directories.
Original game assets must already have been extracted from the supplied ADF.

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/benchmark \
  -duration 20s -warmup 3s -touch \
  -output captures/performance-desktop.json \
  -cpu-profile captures/performance-desktop.cpu \
  -heap-profile captures/performance-desktop.allocs

GOCACHE="$PWD/.cache/go-build" go run ./cmd/benchmark \
  -duration 20s -warmup 3s -touch -original-cadence \
  -output captures/performance-original-cadence.json

GOCACHE="$PWD/.cache/go-build" go tool pprof -top \
  captures/performance-desktop.cpu
GOCACHE="$PWD/.cache/go-build" go tool pprof -top -alloc_objects \
  captures/performance-desktop.allocs
```

The desktop fixture uses two active players with invulnerability and the
highest decoded weapon level to exercise rendering without interrupted runs.
It prewarms actual engine fields, then supplies movement and firing through
the regular Go `Update` and audio paths. Options include `-stage 1`, `-stage 2`,
`-stage 3`, `-final` and `-prewarm` for other scenes. `-unlimited` measures
throughput with VSync disabled; it is not the normal battery-conscious runtime.

For a specifically selected local emulator:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android \
  -target android/arm64 -offline -check -performance -serial emulator-5580
```

This runs the real simultaneous-touch and lifecycle checks, then measures
original-cadence surface presentation, smooth surface presentation, all three
caves and the final encounter. Each scene has two seconds of warmup and ten
seconds of sampling with audio and touch controls active. Scenes use original
maps and native controller/projectile pools, plus diagnostics-only entry
positions and maximum weapon levels. The report is
`captures/android-performance.json`; the smooth screenshot is
`captures/android-smooth.png`. Add `-skip-bind` only when the Go AAR already
matches the current sources. The checker rejects physical-device serials.

For the shorter final-asset Nova check, replace `-performance` with
`-nova-performance`. It verifies a real simultaneous-control session, enters
the two-player fixture, injects Nova through the touch layer, checks the original
charge/counter changes and captures the visible effect while recording timings.
The stress input driver preserves real Nova gestures during this check.

Meaningful tests cover mixed simulation cadences, continued motion between
unchanged native fields, signed early-update phase, camera alignment, stage
and life resets and preservation of engine state. Performance measurements
and temporal smoothing are separate from original behavioral fidelity.

## Review video

The local `captures/android-review-comparison.mp4` shows original-cadence
presentation on the left and smooth presentation on the right. It uses only
real emulator gameplay captures of the same two-player maximum-level surface
fixture. Both sessions advanced native frame 1,150 to 1,800, scroll 735 to
1,060 and ship X100 to X64 during the thirteen-second recording window,
confirming that their simulation timing and input paths match.

The comparison is encoded at 60 FPS, 1280 × 320, for ten seconds. Its underlying
Android screen recordings averaged approximately 57.33 and 58.67 captured FPS
under codec load; the comparison repeats frames where necessary to provide a
common 60 FPS timeline. Encoding rate is therefore not evidence of a strict
60 FPS runtime deadline. Separate timing reports above measure the renderer
without screen recording.

An indicative decoded 24 × 24 terrain-region check found 321 stable intervals
in the original-cadence recording and thirteen in the smooth recording. This
supports the visibly intervening movement, but lossy video encoding and the
luma threshold make it a review aid rather than an Amiga pixel oracle. Raw
clips and codec metadata remain in `captures/android-review-original.mp4`,
`captures/android-review-smooth.mp4` and their JSON sidecars. Recording has no
audio track and uses no substitute game resource.
