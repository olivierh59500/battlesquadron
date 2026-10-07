# Battle Squadron — Go / Ebitengine

A native Go remake of the original 1989 Amiga shoot 'em up. Desktop and Android share the same game, original artwork and native Ron Klaren sound sequencer.

**This is not yet a verified perfect conversion.** Original resources, native
rules and independently verified behavior are distinguished in
[the fidelity report](docs/fidelity.md). In particular, enemy controllers,
campaign transitions and rendering still require wider Amiga comparisons.

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
go run ./cmd/extract -adf '/path/to/original.adf'
./bin/battlesquadron -play -players 2
./bin/battlesquadron -touch
```

Player movement and timers run at 50 updates per second; the original terrain
and NPC loop runs at 25 Hz. Original bitmap pixels use nearest
neighbor scaling. The desktop canvas is 320 × 256; Android adds touch gutters
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
go run ./cmd/android -seed-cache ../kickoff2/.cache/android -offline
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
# Exercise every extracted stage without a graphics environment.
go run ./cmd/verify
# Exercise the original multipart final encounter until its ending signal.
go run ./cmd/verify -final
```

The headless driver enables its verification-only invulnerability option and
uses deterministic input. It checks native stage progression and fixed pool
bounds; it does not establish visual parity or normal-player difficulty.
[Sound verification](docs/sound.md) compares the native sequencer with the
original sound routines using a separate development oracle. That CPU core
is never linked into the application.

`cmd/reverse` runs the installed Ghidra against the extracted original loader.
Its generated projects, bridge scripts and decompiler output remain local.
See [reverse engineering](docs/reverse.md).

