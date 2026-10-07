# Idle expert demonstration

The desktop and Android title menu starts an expert demonstration after 15
seconds without player input. Options, high scores, initials entry, paused games
and ordinary gameplay do not start this timer. The demonstration uses ordinary
native gameplay and the same one-player
joystick commands as the verified expert campaign. A player action interrupts
the demonstration and returns to the menu. Held controls must be released before
they can operate the menu, so the interruption cannot also start an unwanted
game. The interrupted demonstration never submits a high score or changes saved
options.

The runtime does not run the expensive forecasting search in `internal/autoplay`.
It reads `assets/expert.bsinput` once, checks the selected checksum and source
fingerprint, and stores compact input runs. `replay.Cursor.Next` supplies one
ordinary input pair per original PAL field in constant time without allocations.
Rendering retains the smooth presentation used during human gameplay. Each demo
starts a fresh one-player engine with the original default options, independently
of menu difficulty settings. The entire input sequence, including the final 100
ending-payout fields, can be played before the demonstration loops.

The generated recording is control data produced by a native player, rather than
replacement artwork or sound. All displayed graphics and audio still come from
the supplied ADF. The recording is excluded from Git under `assets/`; only Go
generation/playback code and the checksum-only
[`internal/replay/expert.json`](../internal/replay/expert.json) proof are tracked.

## Recreate from the original disk

```sh
make assets
```

This extracts the supplied disk and deterministically regenerates the expert
recording. Forecasting a complete campaign takes several minutes on the
development machine. `cmd/demo` loads extracted files directly and does not
import the application's embedded `assets` package. A clean checkout therefore
has no circular dependency on the generated recording.

To reuse the already selected local verification recording, the faster path
still checks its exact checksum and replays every command through a fresh engine:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/extract
GOCACHE="$PWD/.cache/go-build" go run ./cmd/demo \
  -input .cache/validation/expert.bsinput
```

The generator rejects changed source data, different control inputs, incomplete
progression or a mismatching terminal state. A future intentional gameplay or
expert-player change requires rerunning the full expert validation and reviewing
the checksum-only proof before selecting a different recording.

The selected recording contains 68,401 input fields, including its 100 ending
payout updates. It clears all three caves and the final encounter without a death
using ordinary starting resources. Its native score and terminal state match the
evidence described in [autoplay.md](autoplay.md). This establishes a reproducible
native demonstration; it does not establish a complete matched-input FS-UAE
campaign, pixel parity or audio parity. Those boundaries remain documented in
[fidelity.md](fidelity.md).

## Runtime validation

The full headless presentation check plays every input through the ordinary
demo path, verifies its exact ending digest, loops into a fresh initial state,
and compares saved preferences byte for byte. Other checks cover the full idle
deadline, exclusion of other screens, restored human difficulty/player count,
held-input quarantine, the first fresh pointer after release, Back and suspension.

The real desktop observation uses the normal Ebitengine loop, with audio and
without injected input or smoke mode:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/attractcheck
GOCACHE="$PWD/.cache/go-build" go run ./cmd/attractcheck -touch \
  -output captures/attract-touch-preview.json \
  -capture captures/attract-touch-preview.png
```

Both 2026-10-08 observations started at 750 idle ticks and advanced 250 native
demo fields before capturing an actual frame. Desktop rendering measured
164.60 FPS with a 12.958 ms maximum interval; the touch preview measured
164.78 FPS with an 8.269 ms maximum interval. Reports and images remain ignored.
The checker has a 35-second deadline and reports interruption or lack of focus
instead of forcing a demonstration while a player is using the application.

The idle timeout, expert input sequence and interruption behavior are intentional
remake features. They do not claim to reproduce the original Amiga attract mode.
Physical Android checks and shared-device precautions are described in
[android.md](android.md).
