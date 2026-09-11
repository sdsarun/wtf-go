# Project 10 — Binary File Format Parser

| | |
|---|---|
| **Difficulty** | 3 / 5 |
| **Estimated time** | 6–9 hours |
| **Prerequisites** | [02](02-bit-toolkit.md), [09](09-mini-grep.md) |
| **Builds toward** | [22 – Storage Engine](22-storage-engine.md), [25 – KV Store](25-kv-store.md) |

## Why this project

Real file formats are length-prefixed chunks of little- or big-endian integers with
checksums. Parsing them safely — against **hostile** input — is a core systems skill. You
will use `encoding/binary` both ways, read from an `io.ReaderAt` with `io.SectionReader`,
verify CRC-32s, and then **fuzz** the parser until it provably never panics, hangs, or
tries to allocate 4 GiB because a length field said so.

## Go concepts you MUST use

- [ ] `encoding/binary` — `binary.BigEndian` / `binary.LittleEndian`, `binary.Read` into a
      fixed-layout struct, `ByteOrder.Uint16/Uint32`
- [ ] `io.ReaderAt`, `io.NewSectionReader`, `io.LimitReader`, `io.Discard`,
      `io.ReadFull`, `io.EOF` / `io.ErrUnexpectedEOF`
- [ ] `hash/crc32` — `crc32.MakeTable(crc32.IEEE)`, `crc32.Update`, streaming `crc32.NewIEEE`
- [ ] `encoding/hex` — `hex.Dumper` for unknown chunk payloads
- [ ] `bytes` — signature comparison (`bytes.Equal`)
- [ ] fixed-size struct field layout; why `binary.Read` needs exported, fixed-size fields
- [ ] a **fuzz target** (`FuzzParse`) that must survive arbitrary bytes
- [ ] bounded allocation: never `make([]byte, n)` for an attacker-controlled `n` without
      checking it against the bytes actually remaining

## Background

### PNG (big-endian, CRC-32)

```
signature: 89 50 4E 47 0D 0A 1A 0A        (8 bytes)
then repeating chunks:
  length : uint32  BE   (must be <= 0x7FFFFFFF and <= bytes remaining)
  type   : [4]byte      (ASCII letters; case encodes chunk properties)
  data   : [length]byte
  crc    : uint32  BE   (CRC-32/IEEE over type ++ data)
```

The first chunk must be `IHDR`, the last must be `IEND` (length 0). `IHDR` data is 13
bytes: `width uint32`, `height uint32`, `bitDepth uint8`, `colorType uint8`,
`compression uint8`, `filter uint8`, `interlace uint8`. Color types: `0` grayscale,
`2` truecolor, `3` indexed, `4` gray+alpha, `6` truecolor+alpha.

### WAV / RIFF (little-endian, no CRC)

```
"RIFF"  riffSize uint32 LE   "WAVE"
then subchunks:
  id   : [4]byte
  size : uint32 LE
  data : [size]byte   (padded to an even length with one 0 byte if size is odd)
```

`"fmt "` subchunk (first 16 bytes): `audioFormat uint16`, `numChannels uint16`,
`sampleRate uint32`, `byteRate uint32`, `blockAlign uint16`, `bitsPerSample uint16`.
`"data"` holds the samples.

**Format detection:** first 4 bytes `\x89PNG` → PNG; first 4 bytes `RIFF` (and bytes 8–12
`WAVE`) → WAV; else unknown.

## Requirements

### Functional requirements

1. `binspect [flags] FILE`. Open with `os.Open` (an `*os.File` is an `io.ReaderAt`); `Stat`
   for the size. Detect the format from the magic bytes.
2. Default: print a summary block (format-specific, below) followed by the chunk table.
3. `-chunks` — print only the chunk table (offset, type/id, length, and `crc` = `ok`/`BAD`
   for PNG / `-` for WAV).
4. `-verify` — check every PNG CRC; print `ok` or the first mismatch; exit 1 if any bad.
   For WAV, verify the RIFF size and each subchunk fits; exit 1 if not.
5. `-extract TYPE` — write the raw data of the first chunk of that type to stdout
   (binary). Unknown type → exit 1.
6. `-hexdump TYPE` — `hex.Dumper` the first chunk of that type to stdout.
7. `-json` — emit the summary + chunk list as JSON.
8. Parsing is **streaming over the chunk list** — you may `io.Discard`-skip chunk bodies
   you don't need, and you must never buffer the whole file. Only `IHDR` / `fmt ` bodies
   are read into memory.
9. Every malformed input produces a specific `error:` line and exit 1 — never a panic.

### Exact contract — PNG summary

For `testdata/1x1.png` (a real 1×1 truecolor+alpha PNG: IHDR, IDAT, IEND):

```
format      PNG
dimensions  1x1
bit depth   8
color type  6 (truecolor+alpha)
compression 0
filter      0
interlace   0 (none)
chunks      3
OFFSET  TYPE  LENGTH  CRC
8       IHDR  13      ok
33      IDAT  <n>     ok
<x>     IEND  0       ok
```

### Exact contract — WAV summary

For `testdata/sine.wav` (PCM, mono, 8000 Hz, 16-bit, tiny `data`):

```
format         WAV
audio format   1 (PCM)
channels       1
sample rate    8000
bits/sample    16
byte rate      16000
block align    2
data bytes     <n>
duration       <n/16000>s
subchunks      2
OFFSET  ID      SIZE
12      fmt     16
36      data    <n>
```

### Exit codes

| Code | Meaning |
|------|---------|
| `0` | parsed successfully (and, for `-verify`, everything checks out) |
| `1` | not a recognized format; malformed / truncated; CRC or size check failed; `-extract`/`-hexdump` type not present |
| `2` | usage — no FILE, unknown flag, file cannot be opened |

### Case specification — SUCCESS

| # | Command | key output | exit |
|---|---|---|---|
| S1 | `binspect testdata/1x1.png` | the PNG block above | 0 |
| S2 | `binspect -chunks testdata/1x1.png` | just the `OFFSET TYPE LENGTH CRC` table | 0 |
| S3 | `binspect -verify testdata/1x1.png` | `ok` | 0 |
| S4 | `binspect -json testdata/1x1.png` | valid JSON with `format`, `width`, `height`, `chunks[]` | 0 |
| S5 | `binspect -extract IHDR testdata/1x1.png \| xxd` | 13 bytes: `00000001 00000001 08 06 00 00 00` | 0 |
| S6 | `binspect -hexdump IHDR testdata/1x1.png` | one `hex.Dumper` line | 0 |
| S7 | `binspect testdata/sine.wav` | the WAV block above | 0 |
| S8 | `binspect -chunks testdata/sine.wav` | `12  fmt   16` / `36  data  <n>` | 0 |
| S9 | `binspect -verify testdata/sine.wav` | `ok` | 0 |
| S10 | `binspect testdata/three_idat.png` (2 IDAT chunks) | `chunks 4`, both IDATs listed, `interlace 1 (Adam7)` if set | 0 |

### Case specification — FAILURE (never a panic)

| # | Input | stderr | exit |
|---|---|---|---|
| F1 | `binspect testdata/hello.txt` | `error: unrecognized format (magic 68 65 6c 6c)` | 1 |
| F2 | `binspect testdata/truncated_sig.png` (5 bytes) | `error: file too short for a PNG signature` | 1 |
| F3 | `binspect testdata/truncated_chunk.png` (cut mid-IDAT) | `error: chunk "IDAT" at offset 33: unexpected EOF (need 8192 bytes, 40 remain)` | 1 |
| F4 | `binspect testdata/bad_crc.png` | `error: chunk "IDAT" at offset 33: CRC mismatch (got 0x…, want 0x…)` — but only under `-verify`; the default dump prints `BAD` in the table and still exits 1 | 1 |
| F5 | `binspect testdata/huge_length.png` (length field = 0xFFFFFFF0) | `error: chunk at offset 8: declared length 4294967280 exceeds 2147483647` | 1 |
| F6 | `binspect testdata/no_ihdr.png` (first chunk is `sRGB`) | `error: first chunk is "sRGB", must be "IHDR"` | 1 |
| F7 | `binspect testdata/no_iend.png` | `error: missing IEND chunk` | 1 |
| F8 | `binspect testdata/ihdr_short.png` (IHDR length 10) | `error: IHDR must be 13 bytes, got 10` | 1 |
| F9 | `binspect testdata/riff_lies.wav` (riffSize > file) | `error: RIFF size 999999 exceeds file size <n>` | 1 |
| F10 | `binspect -extract gAMA testdata/1x1.png` | `error: no chunk of type "gAMA"` | 1 |
| F11 | `binspect` | usage | 2 |
| F12 | `binspect nope.png` | `error: open nope.png: no such file or directory` | 2 |

### Error catalogue

| Trigger | Format |
|---|---|
| bad magic | `error: unrecognized format (magic %s)` (first 4 bytes as hex) |
| short signature | `error: file too short for a PNG signature` / `... a RIFF header` |
| chunk EOF | `error: chunk %q at offset %d: unexpected EOF (need %d bytes, %d remain)` |
| CRC | `error: chunk %q at offset %d: CRC mismatch (got %#x, want %#x)` |
| length overflow | `error: chunk at offset %d: declared length %d exceeds 2147483647` |
| wrong first chunk | `error: first chunk is %q, must be "IHDR"` |
| no IEND | `error: missing IEND chunk` |
| IHDR size | `error: IHDR must be 13 bytes, got %d` |
| RIFF size | `error: RIFF size %d exceeds file size %d` |
| extract miss | `error: no chunk of type %q` |

## Suggested milestones

1. `internal/binfmt`: `Detect(r io.ReaderAt, size int64) (Format, error)`.
2. PNG: `parsePNG(r io.ReaderAt, size int64) (*PNG, error)` — signature check, then a
   loop reading `length`/`type`, bounds-checking `length` against `size - offset`,
   optionally CRC-verifying (streaming `crc32` over type+data via a `SectionReader`),
   `io.Discard`-skipping bodies except `IHDR`.
3. `binary.Read` an `IHDR` struct from a `SectionReader`.
4. WAV: `parseWAV` — the RIFF header, subchunk loop with even-padding, `fmt ` struct.
5. Renderers: text summary + `tabwriter` chunk table; JSON structs.
6. `-verify`, `-extract`, `-hexdump`.
7. `FuzzParse(f)` seeded with the good fixtures; run until clean.

## Project layout

```
projects/10-binspect/
  cmd/binspect/main.go
  internal/binfmt/detect.go
  internal/binfmt/png.go
  internal/binfmt/wav.go
  internal/binfmt/render.go
  internal/binfmt/fuzz_test.go
  internal/binfmt/*_test.go
  cmd/binspect/main_test.go
  testdata/            // hand-built minimal fixtures + generator script
  README.md
  Makefile
```

Include a `testdata/gen.go` (`//go:build ignore`) that writes the fixtures, so they're
reproducible and you understand every byte.

## Definition of Done

Extends [README.md](README.md). Additionally:

- [ ] Every S/F row reproduces exactly.
- [ ] `FuzzParse` runs **≥ 5 minutes** with zero crashes, zero hangs, and no allocation
      spike (add a `-fuzz` note in the README with the command).
- [ ] For any input, peak allocation is `O(largest IHDR/fmt chunk + a small buffer)`, not
      `O(file size)` — a 500 MB junk file parses (and errors) in flat memory.
- [ ] `io.ReadFull` / `io.ErrUnexpectedEOF` are used to distinguish "clean end" from
      "truncated".
- [ ] No `panic` reachable from `Parse` — checked by fuzzing and by a
      `TestParseNeverPanics` that throws 10k random byte slices at it.

## Test requirements

- `TestDetect` — PNG, WAV, unknown, too-short.
- `TestParsePNG` — the good fixtures; assert dimensions, color type, chunk offsets.
- `TestParsePNGErrors` — F2–F8 with fixtures.
- `TestCRC` — a known chunk's CRC computed two ways (`ChecksumIEEE` vs streaming) agree;
  a flipped byte is detected.
- `TestParseWAV` / `TestParseWAVErrors` — S7–S9, F9.
- `TestExtract` — S5 bytes exactly.
- `TestBoundedAllocation` — parse a fixture whose length field claims 1 GiB; assert an
  error and that `testing.AllocsPerRun` stays tiny.
- `FuzzParse` — seeded, committed corpus.
- `TestParseNeverPanics`.

## Stretch goals

- Add GIF (LZW-free header + the packed Logical Screen Descriptor flags byte — more bit
  work).
- Decode the `IDAT` stream: `compress/zlib` → raw filtered scanlines → undo the PNG row
  filters → emit a PPM. (Now you've written half an image decoder.)
- `binspect -tree` showing nested RIFF `LIST` chunks.
- A writer: `binspect -strip <types> in.png out.png` that drops ancillary chunks and
  fixes up nothing else (chunks are independent — prove it still opens).
- ELF or ZIP central directory as a third format.

## Self-check questions

1. `binary.Read(r, binary.BigEndian, &ihdr)` — list every requirement on the `ihdr` struct
   for this to work. What happens if a field is `int` instead of `int32`?
2. A chunk header says `length = 0xFFFFFFF0`. Why is `data := make([]byte, length)` a
   denial-of-service bug, and what's the one check that fixes it?
3. PNG is big-endian, WAV is little-endian. On your arm64 Mac, which one matches the CPU,
   and why does `encoding/binary` make that irrelevant to your code?
4. `io.ReadFull` returns `io.ErrUnexpectedEOF` vs `io.EOF` — when each, and how do you use
   the difference to say "truncated" vs "clean end of chunk list"?
5. You verify the PNG CRC by streaming `type ++ data` through `crc32.NewIEEE()`. Why
   stream it instead of `crc32.ChecksumIEEE(append(typeBytes, data...))`?
6. `io.NewSectionReader(file, off, n)` — what does it give you that `io.LimitReader` does
   not, and why does that matter when chunks are not read in order?
7. Your fuzzer found an input that makes the parser loop forever. Without seeing it — what
   class of bug is it, and what invariant in your chunk loop was missing?
