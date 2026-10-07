# Android

The Android version runs the same Go game through Ebitengine's Android view at
50 updates per second. The maintained build tool, game, rendering, audio and
touch controls are Go. Android requires a small native Activity; the Go build
tool generates that lifecycle bridge and its Gradle project under
`android/generated/`. Generated Java, build caches, extracted assets, signing
keys, AARs and APKs remain outside Git.

This follows the [official Ebitengine mobile integration](https://ebitengine.org/en/documents/mobile.html).
No third-party game assets or neighboring project binaries are copied into the
application source tree.

## Requirements

- Go 1.26 or newer and the game's pinned Ebitengine version.
- Java 17 or 21.
- Android SDK platform 36, Build Tools 36.0.0 and platform-tools.
- Android NDK 28.2.13676358, or a compatible NDK selected with `-ndk`.
- Gradle 8.13 with Android Gradle Plugin 8.10.1.
- Locally extracted original assets from the supplied ADF, using the repository's
  extraction command before building.

The tool discovers `ANDROID_HOME`, `ANDROID_SDK_ROOT`, `ANDROID_NDK_HOME` and
`JAVA_HOME`, as well as common Homebrew and Android Studio installations. Gradle
is discovered in `PATH` or existing Gradle 8.13 wrapper distributions in the home
directory and neighboring projects. A fresh machine can select an installed
Gradle executable with `-gradle /path/to/gradle-8.13/bin/gradle`.

## Build

Run from the repository root:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android
```

The default target is ARM64 and the minimum Android API is 23. The command builds
the Go AAR, assembles a signed debug APK and verifies package metadata, API
levels, OpenGL ES 3, the APK signature, archive alignment and native ELF segment
alignment for devices with 16 KiB memory pages. The result is
`bin/battlesquadron-debug.apk`. The disposable development key stays in
`.cache/android/debug.keystore`; it is unsuitable for store releases.

To build both ARM64 and the x86-64 emulator target:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android -target android/arm64,android/amd64
```

To reuse already downloaded dependencies from a neighboring project:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android \
  -seed-cache ../kickoff2/.cache/android -offline
```

The seed operation copies dependency caches into this checkout and skips lock
files. Subsequent builds can use `-offline` alone. The command's caches always
stay in this checkout, including Gradle's user home and Android's user settings.

Additional options include `-prepare` to inspect the generated native project,
`-aar-only` to stop after the AAR, and `-skip-bind` to reuse a previously built
AAR while updating Android packaging. Run `go run ./cmd/android -h` for paths
and options.

## Device behavior

The app uses landscape orientation and an immersive full-screen view. Touch
controls are handled by the same Go input layer as desktop controls, with side
gutters around the original 320 × 256 playfield. Losing focus cancels unfinished
touch gestures. Android suspension pauses gameplay and suspends the Ebitengine
surface; resuming leaves gameplay paused until the player continues. System
Back is routed to the Go game's navigation and closes the Activity at the title.

Install and launch on a connected device only when requested:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android -skip-bind -run
```

Use `-serial DEVICE_SERIAL` to select one device if several are connected. APK
packaging checks do not establish touch, audio or lifecycle behavior on a real
device; those need a separate device or emulator session.

## Emulator checks

The Go build command generates a separate Android instrumentation APK for
runtime checks. It injects two real simultaneous touchscreen pointers, verifies
movement and firing against snapshots published by the Go update thread, then
checks HOME suspension, input cancellation, the paused return to the app and
touch continuation. The checker requires an `emulator-*` transport and refuses
physical device serials:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android \
  -target android/arm64,android/amd64 -offline -check -serial emulator-5580
```

Use `-skip-bind` when checking an unchanged AAR. The separate test APK is installed
only for this option. Evidence is written to the ignored `captures/` directory:
`android-title.png`, `android-touch.png`, `android-paused.png` and
`android-check.json`. A fresh emulator's one-time fullscreen hint is dismissed
before injected touches so that the check measures application input.

The Android 35 ARM64 session in this workspace used a cache-local AVD with
SwiftShader OpenGL ES 3. Simultaneous gestures moved the ship from X112 to X162
while two primary shots were active. After HOME and returning to the app, frame
185 and X164 stayed unchanged; touching the paused game resumed its updates.
These checks establish Android control and lifecycle behavior, not complete
visual or gameplay parity with the Amiga original. The emulator ran with host
audio output disabled, so its successful audio initialization does not establish
audible output quality.
