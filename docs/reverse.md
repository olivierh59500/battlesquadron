# Reverse engineering and original references

The supplied disk was booted in FS-UAE 3.1.66 using the existing Amiga 500
Kickstart installation. Genuine title, attract, high-score, get-ready and
two-player flight screenshots and full chip-memory snapshots are kept under
`.cache/reference/`. They are original game data and therefore excluded from
Git along with native framebuffer captures.

The FS-UAE installation and original ADF are unchanged. A disposable local
copy of the installed emulator and an existing neighboring project's
process-local SDL capture helper were used for reference capture. These
tools and binaries remain in the ignored cache and are not application code.

Ghidra 12.1.4 and OpenJDK 21 were used to inspect the extracted original loader
at Amiga address `$100`, with processor `68000:BE:32:default`:

```sh
go run ./cmd/reverse -addresses 0x100,0x3688
```

The Go command creates a disposable Ghidra project and generates a small
bridge to its scripting API. Disassembly and decompiler output go to
`.cache/reverse/loader.txt`; no generated Java, decompiled original code or
Ghidra project is committed. `-loader`, `-ghidra`, `-addresses` and `-out`
select another extracted payload, installation, routine or output destination.

Actual routine `$3688` calculates the weapon address as
`$1FC8 + weapon*24 + level*4`. Its original template copy establishes the
header and twelve-byte projectile layout used by the native weapon decoder.
Decompiled functions use register-based calling conventions, so the native
translation is checked against the instruction operands and original data.

The public [CrownParkComputing research](https://github.com/CrownParkComputing/BattleSquadron-Amiga)
helped locate original structures. Its native C implementation was not copied.
Its WHDLoad variant differs from this disk: a raw address copied from that
research is not sufficient evidence for this conversion.

Gameplay descriptions can be cross-checked against the
[publisher's Battle Squadron manual](https://cdn.akamai.steamstatic.com/steam/apps/383110/manuals/manual.pdf).
The source disk, captured original pixels and recovered routine operands are
the primary reference for the conversion.
