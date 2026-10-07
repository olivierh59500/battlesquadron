# Original PAL timing audit

The measured opening gameplay separates a 50 Hz player/timer clock from a
25 Hz terrain update. A global divide-by-two of the native clock would slow
player movement and ship timers incorrectly. Terrain and the original NPC
render-loop partition require their own cadence.

This audit used the supplied ADF in local FS-UAE 3.1.66, A500, 512 KiB chip and
512 KiB slow RAM. Saved states were restored into true gameplay before collecting
new state pairs. Warp was disabled. All captures, the emulator and input helpers
are private verification files under `.cache/reference/time-audit/`; they are
excluded from Git and are not build inputs. The previous save slots were backed
up and restored after measurement.

## Physical frame measurement

The UAE `CYCS` payload records cycle unit 512 at offset 4 and the 64-bit cycle
counter at offset 8. The restored game mode is logged as PAL 50 Hz with
`227x312+1`, giving 313 raster lines in these captures. One captured physical
PAL frame therefore contains `512 * 227 * 313 = 36,378,112` stored cycle units.
The independent capture pairs below divide exactly into integer physical frame
counts. This calculation describes this emulator/game mode; it does not assume
that every Amiga PAL mode always uses 313 lines.

The relevant original RAM fields are:

| Field | Actual address |
| --- | ---: |
| Clock word (`A5 + $9058`, signed displacement) | `$1058` |
| Surface progress word | `$9B86` |
| Player-one canonical x | `$4DA2` |
| Player-one rendered x | `$4DA6` |
| Player-one invulnerability counter | `$4DD6` |
| Horizontal camera | `$9B84` |
| Player-two record | `$4EAC` |

## Captured results

| Pair | Stored cycle delta | Physical frames | Clock delta | Scroll delta | Invulnerability delta |
| --- | ---: | ---: | ---: | ---: | ---: |
| Original gameplay states 1 to 2 | 7,821,294,080 | 215 | +214 | +108 | -214 |
| Fresh controlled pair | 1,455,124,480 | 40 | +40 | +20 | -40 |
| Continuous-right pair | 1,273,233,920 | 35 | +36 | +17 | -36 |

The fresh controlled pair is stored as `before.uss`/`after.uss`, with decompressed
`before/` and `after/` folders. Its Clock changes from 6,060 to 6,100; progress
changes from 286 to 306; the invulnerability counter changes from 108 to 68.
This is exactly one Clock/timer step per physical PAL frame and one terrain
pixel per two frames.

For the second controlled pair, the joystick-right key remained held throughout
both saves. Its files are `held-before.uss`/`held-after.uss`. Clock changes from
6,064 to 6,100, and canonical x changes from `$014C` to `$0194`: 72 pixels for
36 original Clock increments, exactly two pixels per Clock step. Rendered x
changes by 85 pixels because the camera also changes by 13. Invulnerability
changes from 104 to 68. Progress changes from 289 to 306.

A Clock delta can differ from the physical-frame interval by one at the saved
CPU/raster phase: the original main render loop batches two player/clock updates.
The 215-, 40- and 35-frame observations establish the rate without interpreting
wall-clock input delays as emulated frame counts. The continuous-input pair also
establishes player speed independently of that one-step phase difference.

## Native implementation boundary

For the measured opening:

- Keep native player movement at two pixels per 50 Hz player tick.
- Keep ship invulnerability and the original Clock at 50 Hz.
- Advance terrain by one pixel every two physical PAL ticks.
- Preserve the original NPC update partition separately; its render-loop cadence
  must not be imposed on player movement or the Clock.

The audit directly measures opening player, timer and terrain behavior. It does
not by itself prove the cadence of every boss state, projectile subclass or a
frame under heavy blitter load. Those paths retain their own instruction and
state comparisons rather than inheriting an unverified global timing factor.
