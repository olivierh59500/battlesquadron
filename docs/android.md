# Android

The Android version runs the same Go game through Ebitengine's Android view at
50 native updates per second, with independently paced smooth rendering at the
display refresh rate. See [performance and presentation](performance.md) for
60 FPS measurements, interpolation and the comparison mode. The maintained
build tool, game, rendering, audio and
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
After 15 seconds without menu input, the expert demonstration starts through
ordinary recorded joystick commands. Any touch, key, controller action or Back
returns to the menu; the waking input must be released before a new action can
start a session. Suspension stops the demonstration and restarts the idle delay.

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
touch continuation. It also checks Nova charge consumption and captures the
original cave and final-battle artwork through diagnostics-only scene fixtures.
The default checker requires an `emulator-*` transport and refuses physical
device serials:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android \
  -target android/arm64,android/amd64 -offline -check -serial emulator-5580
```

Use `-skip-bind` when checking an unchanged AAR. The separate test APK is installed
only for this option. Evidence is written to the ignored `captures/` directory:
`android-title.png`, `android-touch.png`, `android-paused.png`, `android-nova.png`,
`android-cave.png`, `android-final.png` and `android-check.json`. A fresh
emulator's one-time fullscreen hint is dismissed
before injected touches so that the check measures application input.

The checker also waits for the real menu idle delay, observes autonomous expert
gameplay, measures ten seconds of its actual Update/Draw path, and holds a touch
over the menu's start button while dismissing the demonstration. It verifies
that the held wake touch cannot start a session, then releases it and starts
through a fresh touch. This adds `android-demo.png`, `android-demo-wake.png` and
`android-demo-performance.json` to the local evidence.

## Authorized physical-device checks

Use the separate `-physical-check` option only after the device owner has
authorized installation and runtime checks. An explicit physical serial is
required; this option never selects a device implicitly:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android \
  -offline -physical-check -serial 67081JEA300033
```

Physical checks install or update only `com.olivierh.battlesquadron` and its
instrumentation package with `adb install -r`. They preserve application data
and leave global device settings, other packages and their data alone. Unlike
emulator checks, they do not modify fullscreen-hint settings. Optional
`-performance` and `-nova-performance` measurements also work with this explicit
opt-in. Each run has a separate ignored directory beneath `captures/pixel-10a/`,
named with the APK checksum prefix and UTC timestamp. Its `device.json` records
the full tested APK checksum, serial, Android version and page size; failed
runs cannot inherit screenshots or measurements from an earlier success.

The foreground test window is limited to four minutes. A cooperative lease at
`$TMPDIR/codex-android-device-DEVICE_SERIAL.lock/owner.txt` identifies the owner
process and expiry; the checker refuses an existing lease and never removes a
foreign owner record. Other agents can use the same lease convention. Before
installation, an unavailable foreground causes the checker to stop. Before
each injected gesture, state assertion and screenshot, instrumentation checks
that Battle Squadron still owns the foreground. If another app takes over, it
stops injecting input and reports the interruption. The checker does not clear
logcat, force-stop unrelated apps, uninstall packages or change display refresh
settings.

## Pixel 10a validation

On October 8, 2026, the shared USB Pixel 10a ran the attract-mode checks on
Android 17 / API 37, with a 1080 × 2424 display at its existing 60 Hz setting
and 4096-byte memory pages. The complete hardware-checked APK SHA-256 was:

```text
3ff1cbf3686c1ea56b814a3f0166193620c8a79fce40a85a6599133e858b9588
```

The actual menu waited fifteen seconds, entered the recorded expert run, and
advanced through ordinary native inputs. HOME and resume restored the human
menu and restarted its idle delay. After the next demonstration began, holding
a touch over Start returned to the menu without starting a session; releasing
it and touching again started human gameplay. Simultaneous movement and fire,
paused HOME/resume, cancelled gestures and Nova consumption also passed. Cave
and final-encounter rendering were exercised through the explicit diagnostic
fixtures.

The real demonstration measured 59.93 FPS, 50.01 native updates per second and
59.83 terrain motion changes per second. Six scene samples measured
59.93–59.97 FPS; original-cadence terrain changed at 25.11 Hz while smooth
surface terrain changed at 59.83 Hz. Each steady sample lasted ten seconds
after warmup. Frame-interval p99 values were approximately 24–25.3 ms, with a
maximum of 27.3 ms. These are average 60 Hz results, not a guarantee that every
frame meets a 16.67 ms deadline. The device's 4 KiB runtime does not independently
exercise the APK's separately verified 16 KiB alignment.

The exact-build reports and screenshots are local under
`captures/pixel-10a/3ff1cbf3686c-20261007T230822.740344000Z/`. A separate retry
also demonstrated that foreground takeover stops input injection. Unrelated
packages and their data were preserved, as were the device settings.

The final local APK adds only a staff-card footer presentation fix after that
complete hardware run: the human Start prompt is hidden while the demonstration
caption is visible. Its package, signature, ABI and alignment checks passed,
with SHA-256:

```text
6d08bbf48bea5b17cbb571f094b06bb08915472b0913d00ce7630a44963b1599
```

The shared phone became busy again, so this caption-only build was not
reinstalled there. The hardware results above refer specifically to the
`3ff1cbf3686c…` build; the latest installable artifact is
`bin/battlesquadron-debug.apk`.

Add `-performance` to the emulator check command to measure actual frame rates,
CPU work, allocation, native update rates and intervening terrain movement in
the surface, all three caves and final encounter. Results remain local in
`captures/android-performance.json`; the measured smooth view is captured in
`captures/android-smooth.png`. The run includes an original-cadence surface
comparison and uses real Go rendering, audio and touch-control paths. It takes
approximately ninety seconds after installation. These measurements describe
the selected emulator rather than guaranteeing every physical device's frame
deadline.

Use `-nova-performance` instead of `-performance` for a shorter targeted check
of the restored original Nova hardware artwork. It injects the actual Nova
touch during the two-player stress fixture, checks the original charge and
active counter, captures `android-nova-restored.png` and writes
`android-nova-performance.json`. See [original Nova artwork](nova.md) for
source addresses, checksums and the final APK's measured timing boundaries.

The Android 35 ARM64 session in this workspace used a cache-local AVD with
SwiftShader OpenGL ES 3. The corrected original entry finished at Y118 after
130 PAL fields. Simultaneous gestures moved the ship from X64 to X114
while a primary fire bank was emitted. Cumulative fire events are checked because
projectiles can leave the screen before a later snapshot. After HOME and
returning to the app, frame 171 and X116 stayed unchanged; touching the paused
game resumed its updates. The Nova touch consumed one of the original three
charges and produced twelve active Nova projectiles.
These checks establish Android control and lifecycle behavior, not complete
visual or gameplay parity with the Amiga original. The emulator ran with host
audio output disabled, so its successful audio initialization does not establish
audible output quality.
