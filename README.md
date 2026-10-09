# Battle Squadron

A remake of the original 1989 Amiga shoot 'em up, built in Go with Ebitengine
for desktop and Android. Fly solo or with a second pilot, upgrade your weapons,
and battle across the surface and three underground zones.

## Screenshots

| Title screen | Surface combat | Underground combat |
| --- | --- | --- |
| [![Battle Squadron title screen](https://www.malakhsoftware.com/img/projects/battlesquadron-title.png)](https://www.malakhsoftware.com/img/projects/battlesquadron-title.png) | [![Battle Squadron surface combat](https://www.malakhsoftware.com/img/projects/battlesquadron.png)](https://www.malakhsoftware.com/img/projects/battlesquadron.png) | [![Battle Squadron underground combat](https://www.malakhsoftware.com/img/projects/battlesquadron-cave.png)](https://www.malakhsoftware.com/img/projects/battlesquadron-cave.png) |

## Presentation

Watch the [4 minute 44 second presentation on Malakh Software](https://www.malakhsoftware.com/games.html#battlesquadron),
with English subtitles, original music and expert gameplay through the final
encounter.

## Features

- One or two simultaneous players, with keyboard and gamepad controls.
- Android touch controls for movement, fire and Nova at the same time.
- Four weapon families, six power levels, collectible capsules and building wrecks.
- Surface combat, three underground zones and multipart bosses.
- Original artwork, music and sound reconstructed from the Amiga game data.
- Smooth movement at the display refresh rate, with the original gameplay timing.
- An expert demonstration after 15 seconds of inactivity on the title screen.
- Difficulty settings, saved preferences and a local high-score table.

## Original game data

You need an Amiga ADF disk image of Battle Squadron to reconstruct the game
resources. The disk image and extracted graphics, maps and audio are not
included in the repository.

The [Planet Emulation Amiga ADF catalogue, letter B](https://www.planetemu.net/roms/commodore-amiga-games-adf?page=B)
can help you find Battle Squadron. The extractor currently supports the verified
disk revision with this SHA-256 checksum:

```text
de335a312577f757ea699bd9a19162df9f5e89d83b7ca8a8deeac7ff0116609f
```

Other disk revisions may differ. Extract the `.adf` first if it comes in an
archive, then supply its path when preparing the resources. The file can remain
anywhere on your machine.

## Build and run

Requires Go 1.26 or newer and a desktop graphics environment.

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/extract -adf '/path/to/Battle Squadron.adf'
GOCACHE="$PWD/.cache/go-build" go run ./cmd/demo
make run
```

The second command generates the verified expert demonstration and takes about
five minutes on the development machine. Both commands are required before the
first build. Subsequent builds reuse the generated resources.

```sh
make build
./bin/battlesquadron
./bin/battlesquadron -play -players 2
./bin/battlesquadron -touch
```

The desktop picture keeps the original 320 × 256 layout. Android adds separate
touch gutters without stretching the playfield. Use `-original-cadence` to
compare discrete movement with the default smooth rendering.

## Controls

| Input | Action |
| --- | --- |
| F1 / F2 on the title | Select one / two players |
| Enter / fire on the title | Start |
| Arrows + Space / Right Ctrl | Player one movement and fire |
| X / Right Shift | Player one Nova |
| WASD + C / Left Ctrl | Player two movement and fire |
| V / Left Shift | Player two Nova |
| Gamepad stick / D-pad + primary / secondary button | Movement, fire and Nova |
| P / Pause / gamepad Start | Pause or resume |
| Escape / Android Back | Close options, pause, then return to the title |
| F3 / F4 | Toggle sound effects / music |
| F5 on the title | Difficulty options |
| H on the title | High-score table |
| M | Toggle all audio |
| F11 | Fullscreen |

Touch controls allow simultaneous movement, fire and Nova. Pause, Sound and
Menu buttons remain accessible. A second Android player can use an external
keyboard or gamepad. Leaving the application pauses a human game and releases
unfinished gestures.

## Expert demonstration

Leave the title screen idle for 15 seconds to watch the expert play. Any
keyboard, mouse, gamepad or touch action returns to the menu. Release that
control before starting a human game.

The demonstration follows a complete campaign, collects bonuses, attacks
buildings and reaches the final encounter without losing a ship. It loops after
the ending and never submits its score or changes your selected difficulty.
Options, high-score entry and running games are not interrupted.

## Android

```sh
make android
```

The APK is generated at `bin/battlesquadron-debug.apk`. See
[Android setup and Pixel 10a validation](docs/android.md) for SDK requirements,
touch controls and device checks.

## Development

```sh
make test
make vet
```

The remake is playable; complete behavioral and pixel parity with the original
remains under evaluation. See the [fidelity report](docs/fidelity.md),
[performance measurements](docs/performance.md) and
[expert campaign validation](docs/autoplay.md) for the current checks and limits.

The [presentation tool](docs/showcase.md) recreates the subtitled video from
native gameplay and original audio. Implementation details and reference tooling
are documented in [reverse engineering](docs/reverse.md).
