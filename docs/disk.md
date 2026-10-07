# Original disk and native resource reconstruction

The verified source is the 880 KiB OFS disk in `previous/`:

`Battle Squadron - The Destruction of the Barrax Empire! (1989)(Innerprise)[cr CP].adf`

Its SHA-256 is `de335a312577f757ea699bd9a19162df9f5e89d83b7ca8a8deeac7ff0116609f`.
The extractor rejects other revisions before writing anything. Resource layouts
and relocated loader addresses have been verified against this exact revision.

Run `go run ./cmd/extract` to recreate local resources. All extraction,
decompression, planar conversion and provenance generation are implemented in Go.
The ADF, extracted files, PNGs, audio data and emulator captures remain excluded
from Git. The generated `assets/provenance.json` lists every original file's
header block, ordered data blocks, payload offsets, byte count and SHA-256, plus
the decoded modules' checksums and load addresses. `assets/manifest.json` lists
the graphics available to the native renderer.

## File system

The boot block is standard `DOS\0`; the volume is `BS` and its root block is 880.
The directory contains 27 files including the three `s/` startup files. OFS
blocks are 512 bytes; each data block has a 24-byte header and at most 488 payload
bytes. The implementation verifies every header and data block checksum, block
bounds, sequence numbers, owner pointers, chain termination and declared sizes.
Unsafe path components and cyclic directory/data chains fail extraction.

The filesystem startup launches `BattleDOS`. Its executable wrapper contains
cracker metadata and a relocated SPIK unpacker. The embedded game-loader stream
starts at executable byte offset `$21E`, after the wrapper. The recreated loader
is 64,768 bytes at original Amiga address `$100`; its first instructions are
`BSR.W $150` and `JMP $400`. No cracktro or cracker message is used by the native
presentation.

## SPIK compression

A SPIK resource stores its signature, compressed byte count and decoded-body
byte count as three big-endian longwords. The following 32 bytes hold the final
32 decoded bytes. The compressed stream starts at byte 44. Its control bits and
literal bytes share one cursor. Literal runs alternate with forward overlapping
LZ matches; a match can also appear without a preceding literal run.

The Go implementation translates the actual 68000 decoder disassembled from
`BattleDOS`, executable offsets `$126` through `$1BE`. The original in-place
routine can emit one byte beyond the decoded body before writing the saved
32-byte tail over that location. The Go decoder reproduces that boundary while
rejecting larger overruns, invalid references and truncated streams.

Selected decoded modules:

| Resource | Original load address | Decoded bytes |
| --- | ---: | ---: |
| Embedded loader | `$100` | 64,768 |
| `loddat` | `$10000` | 83,696 |
| `lods0f` | `$2E508` | 62,200 |
| `lods0s` | `$3D800` | 26,624 |
| `lods0t` | `$44000` | 106,496 |
| `lodst1` | `$2E89A` | 194,406 |
| `lodst2` | `$2E4C0` | 195,392 |
| `lodst3` | `$2E840` | 194,496 |
| `lodgam` | `$246F0` | 38,108 |
| `lodcom` | `$3D800` | 20,218 |
| `lodmus` | `$3D800` | 22,470 |
| `lodint` | `$62000` | 40,000 |

The cracker's SPIK loader returns before the older BOND loader's stage-one byte
reversal. Consequently the decoded `lodst1` already has runtime order; applying
that older reversal a second time corrupts its graphics. Original overlay load
addresses remain fixed while many loader tables moved compared with other PAL
and WHDLoad revisions.

## Graphics

The title is the original `lodint` bitmap: 320 by 200 pixels, five bitplanes,
8,000 bytes per plane. Its menu palette is the 32 RGB4 words at decoded loader
file offset `$169E`.

Player ships use fourteen 120-byte hardware sprite strips at the start of
`loddat`. Two adjacent 16-pixel strips form each 32 by 30 frame; each scanline
interleaves its two plane words. Seven frames represent the ship's bank angles.
The original first-player colours are `$FDD/$889/$225`, and the second player's
are `$EFD/$C94/$521`. Frame 3 is neutral. The score font reads one byte per
8-pixel glyph row, with ten bytes per glyph and its ASCII base at `loddat + $550`.

The surface and three cave terrain streams each contain 512 rows of 24
big-endian words at `$44000`. The tile bank starts at `$4A000`. A map word is a
word offset into that bank, so the byte offset is twice its value. Tiles are
16 by 16, use five planes of 32 bytes each, and occupy 160 bytes. Full native
maps are 384 by 8,192 pixels. Some unused tile-bank spans also contain unrelated
original overlay data; the game only references its valid terrain spans.

Scenery templates supply their graphic pointer, frame stride, dimensions and
initial state. The extractor follows the original tile checks to those records.
Sprite metadata includes the template address because several templates share
an object type while selecting different original artwork. Masked flying
objects use their separate original five-plane colour and one-plane mask
pointers. Their 32-byte descriptor table starts at `$CCE0` in this disk's loader.
Primary weapons use the hardware sprite lookup at `$C61C` and the height from
the original twelve-byte projectile record. Stage metadata distinguishes
artwork that is replaced by a later overlay.

The original startup pre-fills and clears the terrain ring, selects map pointer
`$49E20` and sets progress `$A0`. Its initial partially visible cloud band and
starfield require that startup state; a generic view of the full map does not
reproduce the startup composition by itself.

## Verification

`go test ./internal/source` exercises literal/match interleaving, overlapping
copies, the saved tail, damaged streams, malformed sizes, incorrect checksums,
unsafe file names, invalid block bounds and cyclic links. With the local ADF
present it also verifies known SHA-256 values for the decoded loader, `loddat`
and `lods0t`.

The optional local FS-UAE state comparison verifies every resident `lods0t`
byte at `$44000` (106,496 bytes). It also verifies resident `loddat` outside six
runtime pointer slots at `$1737A..$17391` and the later `lodgam` overlap beginning
at `$246F0`. These emulator files are verification evidence, not build inputs.
All build assets are reproducible directly from the ADF.

## HUD, pickups and staff card

The boxed Nova counter is a Copper graphic, not a font character. Its original
13 rows write `SPR0DATA`/`SPR0DATB` and change three sprite colours on each raster
line. The native `hud_nova_1` and `hud_nova_2` images preserve the separate left
and right palette gradients. Both are 16 by 13 pixels.

Masked flying descriptor 5 is the collectable capsule. Its 12 contiguous frames
start at colour pointer `$12890` and mask pointer `$12930`, with a 192-byte frame
stride. Weapon subtypes 0, 2, 4 and 6 select pairs 0/1, 2/3, 4/5 and 6/7. Nova
subtype 10 selects the original boxed M frames 10/11. Its animation frame is
`subtype + ((globalFrame >> 2) & 1)`.

The original post-battle staff card is `lodtem`: 640 by 200 pixels, four
16,000-byte planes. The loader selects its 16-colour grayscale preset at file
offset `$15FE`. It shows the original staff portraits in an open book. Its
Amiga hires aspect is reproduced on a 320-pixel logical display by halving only
the horizontal size. `scene_lodtem` exposes this card directly. `scene_lodlod`
also exposes the original game's 320 by 208 splash landscape using the palette
at loader file offset `$179E`.

The opening starfield is already present in the surface map. Tile words `$280`,
`$2D0`, `$320`, `$370` and `$3C0` contain the original sparse coloured stars; `$A50`
is completely blank. The initialized map pointer `$49E20` corresponds to source
row 502. A visible source coordinate of `8032 - liveScroll` therefore reproduces
the original opening terrain/starfield boundary. The game's progress counter
starts at 160, so this also equals `mapHeight - progress` before later transitions.

## Surface entrances and final boss

The actual loader entrance checks at `$3026`, `$303E` and `$3056` compare
progress with `$F10`, `$1490` and `$1DD0`. Their original column counters 11,
9 and 12 produce world x coordinates 192, 224 and 176 using `(23-counter)*16`.
All three select the portal template at `$2F40`, type 39, whose original
`ENTER HERE` artwork at `$35B00` has two 32 by 32 states.

The original portal controller watches positions in a 32-pixel box for ten
consecutive updates. Every live ship must be inside; the left, right and top
edges are inclusive while the bottom edge is excluded. The native campaign
helper preserves that rule and separately stores the surface position, active
cave and three clear bits. Restoring the entered surface position makes native
routing functional; the original fade, ring prefill and earlier return-position
sequence remain a distinct timing detail to verify.

`lodfin` is final-boss artwork, not a flat ending bitmap. Every decoded byte is
accounted for by its contiguous original banks:

| Component | Colour pointer | Mask pointer | Frame geometry | Frames |
| --- | ---: | ---: | --- | ---: |
| Body | `$44000` | `$453B0` | 96 by 28, colour stride 336 | 3 |
| Head | `$457A0` | `$50060` | 96 by 48, colour stride 576 | 15 |
| Left gun | `$50BA0` | Last plane of each block | 80 by 32, stride 320 | 12 |
| Right gun | `$565A0` | Last plane of each block | 80 by 32, stride 320 | 12 |

The body has three five-plane colour frames followed by three one-plane masks.
The head has fifteen colour frames and five shared/special masks: ordinary
frames 0–10 share `$50060`, while frames 11–14 select subsequent 576-byte masks.
Each gun frame stores five colour planes followed by its one-plane mask. These
spans sum to exactly 98,208 bytes. The finale's actual palette selector points
to Amiga `$163E`, decoded loader file offset `$153E`. Native atlas IDs are
`final_body`, `final_head`, `final_gun_left` and `final_gun_right`.
