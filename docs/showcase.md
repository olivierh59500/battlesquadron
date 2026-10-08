# Presentation video

The local presentation is `captures/battlesquadron-presentation.mp4`: 4 minutes
44.033 seconds, 1280 × 960, 17,042 H.264 frames at 60 FPS, with stereo AAC audio
at 44.1 kHz. English captions are burned into the picture using the original
bitmap font in a separate editorial strip. The matching `.srt` sidecar is
available for editing. No automatically enabled subtitle track duplicates the
visible captions.

The first 53 seconds show the real native title, player selection, options,
high-score table and the full fifteen-second idle activation. The remainder
uses selected intervals of one verified, death-free expert campaign: weapon
capsules, buildings, the first cave entrance, cave bosses, both Nova uses and
the final encounter. The original staff card closes the video. Cuts are edited
highlights; the footage does not claim to complete the game in four minutes.

## Recreate

After reconstructing the original resources and expert input with `make assets`:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/showcase
GOCACHE="$PWD/.cache/go-build" go run ./cmd/showcase -validate
```

The recording command requires a desktop graphics environment and FFmpeg with
H.264/AAC encoding. A short layout/audio preview can be generated separately:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/showcase -limit 12s \
  -output captures/showcase-preview.mp4
```

Native simulation and sound events advance at their original 50 Hz cadence.
The recorder supplies one explicit virtual clock to both presentation history
and drawing, then captures the production final viewport at 60 video frames
per second. Rendering elapsed wall time does not affect gameplay speed or
interpolation. The output's encoded rate is not a runtime FPS benchmark.

The same native sound driver supplies PCM directly; the tool does not record
a microphone, system audio, another application or the shared Pixel. Omitted
input intervals still run every native update and original audio sample before
the next edit, preserving earned weapons, score, lives and Nova stock. No enemy
health, player resources or campaign flags are altered for filming.

Reusable RGBA buffers stream directly to FFmpeg. Only compressed video and a
native PCM WAV are staged under `.cache/showcase/`; the process does not write
thousands of full-resolution frame images. MP4, WAV, screenshots, subtitle
sidecars and diagnostic JSON remain outside Git. The Go tools, chapter/caption
plan and tests are maintained in the repository.

## Verification

The complete render reached input position 68,401, native frame 68,301 and the
ending, with zero deaths, five ships, weapon three at level five, seven Novas
and score 2,473,920. A separate native validation executed the same chapter
steps and all omitted inputs, including original PCM progression, and matched
the expert's complete terminal state digest:

`467af6b438b0b2a5681233d3490923d11e066ebf5140e9a26db4cb46e627223b`

The initial capture preceded addition of the digest field to its report. Its
JSON explicitly identifies the subsequent verification rather than claiming
that the digest was observed during that original render. Future complete
captures enforce the digest directly.

Inspection covered the title, options, high scores, weapon upgrades, cave boss,
Nova, final encounter and ending frames. A decoded 64 × 64 terrain crop from
80–82 seconds had 119 visibly moving intervals in 120 frames, with no stable
interval at the 0.5 mean-luma threshold. This checks intervening video motion;
it does not establish original Amiga pixel parity. The final audio was decoded
and checked for nonzero signal across the full duration.

Final MP4 SHA-256:

`24a0b4b5e10c93a37956f1d3f2ed683bcd21d3974371e5a57d68c9e21f84b13c`

Original asset, behavior and analog sound boundaries remain documented in
[fidelity.md](fidelity.md). This presentation shows the native reconstruction.
