# Battle Squadron — Go / Ebitengine

A native Go remake of the original 1989 Amiga shoot 'em up. Desktop and Android share the same game, original artwork and native Ron Klaren sound sequencer.

**This is not yet a verified perfect conversion.** Original resources, native
rules and independently verified behavior are distinguished in
[the fidelity report](docs/fidelity.md). Complete matched-input campaigns and
framebuffer comparisons against the Amiga remain outstanding.

## Reproduce and run

Requires Go 1.26 or newer and a desktop graphics environment.

```sh
make assets
make run
make build
./bin/battlesquadron
```

The Go extractor reads the original disk filesystem and reconstructs the game's artwork, sound and data tables from an external ADF file. It uses Go's standard library. No original disk, graphics, samples,
executable bytes or generated asset manifest is committed. A fresh checkout
needs the supplied ADF before extraction and compilation.

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/extract -adf '/path/to/original.adf'
GOCACHE="$PWD/.cache/go-build" go run ./cmd/demo
./bin/battlesquadron -play -players 2
./bin/battlesquadron -touch
```

Player movement and timers run at 50 updates per second; the original terrain
and NPC loop runs at 25 Hz. The default renderer interpolates positions at the
display refresh rate, with a 40 ms visual delay, to make movement smoother at
60 Hz and above. Original textures use nearest-neighbor sampling into a denser
viewport and linear final sampling to retain fractional movement. Use
`-original-cadence` for discrete source timing and nearest-neighbor presentation.
See [performance and smooth rendering](docs/performance.md) for measurements
and exact limits. The desktop canvas is 320 × 256; Android adds touch gutters
to a 480 × 256 canvas without stretching the original picture.

| Input | Action |
| --- | --- |
| F1 / F2 on the title | Select one / two players |
| Enter / fire on the title | Start |
| Arrows + Space / Right Ctrl | Player one movement and fire |
| X / Right Shift | Player one Nova |
| WASD + C / Left Ctrl | Player two movement and fire |
| V / Left Shift | Player two Nova |
| Gamepad stick / D-pad + primary / secondary button | Corresponding player's movement, fire and Nova |
| P / Pause / gamepad Start | Pause or resume |
| Escape / Android Back | Close options, pause, then return to the title |
| F3 / F4 | Toggle original effects / music |
| F5 on the title | Difficulty options |
| H on the title | Original high-score table |
| M | Toggle all audio |
| F11 | Fullscreen |

Touch controls support simultaneous movement, fire and Nova. The separate
Pause, Sound and Menu controls remain accessible. Losing focus pauses gameplay
and releases unfinished gestures. The second player can use an external
keyboard or gamepad on Android.

After fifteen foreground seconds without input on the title screen, the expert
demo starts automatically on desktop and Android. Any keyboard, mouse, gamepad
or touch action returns to the menu; release that gesture before starting a
human game. Options, high-score entry, paused sessions and running human games
are not interrupted. Backgrounding the application dismisses the demo and
resets the idle interval.

The demo replays the verified complete expert campaign through ordinary native
inputs, loops after the ending, and never saves its scores or changes selected
players or difficulty. Forecasting runs only in the Go resource-generation
tool, so watching the demo adds no search workload to the frame loop.
`make assets` reconstructs the original artwork/sound and regenerates the ignored
expert input file; the full forecast currently takes about five minutes.
See [attract-mode generation and verification](docs/attract.md).

Settings and high scores are saved atomically in the OS configuration directory
under `battlesquadron`, or the directory selected by `-data-dir`. The twelve
original high-score entries provide the initial table. Qualifying scores accept
three initials using the keyboard or by tapping the letters and Save prompt.

The original four weapon families and their six levels are decoded from the
game's actual loader. Initial weapon, lives, projectile count, bullet speed
and firing delay can be changed in the options screen.

## Android and verification

```sh
make android
# Reuse this machine's existing downloaded Android dependencies.
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android -seed-cache ../kickoff2/.cache/android -offline
```

The generated APK is `bin/battlesquadron-debug.apk`. The maintained build tool
is Go; it generates the small Android Activity and Gradle project required by
Ebitengine inside an ignored directory. See [Android setup](docs/android.md).

```sh
make test
make vet
./bin/battlesquadron -mute -smoke 260 -capture captures/native-surface.png
./bin/battlesquadron -mute -touch -smoke 260 -capture captures/native-touch.png
./bin/battlesquadron -mute -screen title -smoke 20 -capture captures/native-title.png
```

The capture option saves the actual Ebitengine framebuffer before exiting.

```sh
# Play the entire ordinary campaign through joystick input only.
GOCACHE="$PWD/.cache/go-build" go run ./cmd/verify -mode expert
# Replay its recording through a fresh engine without planning.
GOCACHE="$PWD/.cache/go-build" go run ./cmd/verify -mode replay \
  -input .cache/validation/expert.bsinput -report .cache/validation/replay.json
# Inspect the final encounter using explicit diagnostic starting resources.
GOCACHE="$PWD/.cache/go-build" go run ./cmd/verify -mode inspect -final
```

The expert player starts with the normal weapon, three ships and three Nova
charges. It forecasts private copies and supplies ordinary input, then checks
a fresh replay against complete mutable-state fingerprints. It collects
capsules and wrecks, attacks flying enemies and buildings, enters all three
caves and fights the final encounter. Planning is a development workload,
separate from runtime performance. See [expert validation](docs/autoplay.md).
The explicitly named `inspect` mode enables invulnerability and a maximum-level
weapon; its reports cannot establish ordinary progression. Native input replay
does not establish original Amiga behavior or pixel parity.
[Sound verification](docs/sound.md) compares the native sequencer with the
original sound routines using a separate development oracle. That CPU core
is never linked into the application.

`cmd/reverse` runs the installed Ghidra against the extracted original loader.
Its generated projects, bridge scripts and decompiler output remain local.
See [reverse engineering](docs/reverse.md).

`cmd/reference` reads exact FS-UAE input recordings and compressed USS states
for bounded original/native comparisons. The opening comparison matches 29
selected scalars over 36 PAL fields; it does not establish complete campaign or
pixel parity. See [original reference checkpoints](docs/reference.md).

