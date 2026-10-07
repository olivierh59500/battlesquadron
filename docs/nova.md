# Original Nova artwork

The Nova's four visible graphics were missing from the original asset catalogue.
The extractor enumerated the twenty-four ordinary weapon templates and the two
orb entries, while the original twelve Nova templates live in a separate bank.
That omission made both the eight emitted rays and the Nova primary projectiles
request absent graphic IDs 84 through 87.

The supplied loader's ordinary hardware-sprite pointer table is at `$C61C`.
Its entries for graphics 84, 85, 86 and 87 point to `$CB58`, `$CB98`, `$CBD8`
and `$CC18`. Each frame is exactly 64 original bytes: sixteen scanlines of
interleaved two-plane 16-pixel hardware data. The twelve original Nova shot
templates independently specify height sixteen for those four graphics.
Their native 48-pixel collision width remains separate from the 16-pixel
hardware bitmap width; extraction does not change damage or collision geometry.

The extractor now includes `DecodeNova`'s graphic/height records in the ordinary
hardware catalogue. It reads the displayed RGB4 values from the original Copper
color registers 25 through 27, currently `$FDD`, `$889` and `$225`; index zero
is transparent. These match the previously used ordinary hardware colors, but
their provenance is now the loader's actual color words. No replacement
particle, generated frame or substitute color palette is used.

`internal/source/nova_graphics_test.json` contains only source-byte and decoded
pixel SHA-256 fingerprints for the four frames. Its test reads the supplied ADF,
reconstructs the loader and resident bank, runs the real catalogue extraction,
and checks that all four manifest entries retain their original addresses,
16 × 16 geometry and fingerprints. The PNGs and manifest remain ignored local
assets and are recreated with `make assets`.

The desktop/Android renderer already uses these graphic IDs for both paths;
the restored resources now make them visible. Artwork loading rejects an old
catalogue missing these four entries rather than silently drawing no Nova.

Actual runtime captures are `captures/native-nova-restored.png` on desktop and
`captures/android-nova-restored.png` on Android. The Android 35 ARM64 SwiftShader
session on 2026-10-08 passed real touch, pause/resume and Nova checks, including
charge three to two, twelve active projectiles and original counter 230 at the
capture. A separate 9.55-second measurement with visible Nova, both players,
audio and touch controls observed 59.90 FPS, Draw CPU p95 0.056 ms, Update CPU
p95 0.022 ms, frame-interval p95 17.403 ms and maximum interval 31.504 ms.
It includes the screenshot checkpoint and does not prove an every-frame 60 FPS
minimum. Details remain in `captures/android-nova-performance.json`.

The final signed debug APK containing this restoration and the verified source
single-player enemy-health correction is `bin/battlesquadron-debug.apk`, with
SHA-256 `95fe1ff07d2be0c855cd0be5391b2f1582696d091d4520e89f203b6d98648945`.
Reproduce its targeted emulator checks with:

```sh
GOCACHE="$PWD/.cache/go-build" go run ./cmd/android \
  -target android/arm64 -offline -check -nova-performance -serial emulator-5580
```

These checks establish source-strip and decoded-bitmap integrity. They do not
establish the complete original composited framebuffer, sprite multiplexing,
per-scanline palette interaction, integrated Nova collision ordering or every
original presentation frame.
