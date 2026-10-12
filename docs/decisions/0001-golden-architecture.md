# 0001: Golden pages and processor architecture

Date: 2026-09-27
Status: decided

## Question

Golden pages are compared byte for byte. The original `graphite-report` ran
on one processor architecture only. Do `linux/amd64` and `linux/arm64`
produce the same bytes under Typst 0.15.1?

## Method

The examples of `graphite-report` 1.0.0 were rendered with the pinned image
on both platforms, with the four fixed flags, as PNG at 144 PPI and as PDF.

- Image: `ghcr.io/typst/typst:0.15.1`
- Index digest:
  `sha256:032e292249bcd378480cc7c142cfa324b63ef8aadeb88d7e7230320c4c9c422f`
- Host: macOS on ARM, Docker 29.8.0; `linux/amd64` ran under emulation.

```bash
docker run --rm --platform "$PLATFORM" \
  -v "$PWD/.cache/arch:/work" \
  ghcr.io/typst/typst:0.15.1 compile \
  --root /work/pkg/local/graphite-report/1.0.0 \
  --package-path /work/pkg \
  --font-path /work/pkg/local/graphite-report/1.0.0/assets/fonts \
  --ignore-system-fonts --ignore-embedded-fonts \
  --format png --ppi 144 \
  "/work/pkg/local/graphite-report/1.0.0/examples/$NAME.typ" \
  "/work/out/${PLATFORM#linux/}/$NAME-{p}.png"
```

`PLATFORM` was `linux/amd64` and `linux/arm64`. `NAME` was `complete`,
`custom-running-elements`, and `minimal`. The PDF of `complete` was compiled
the same way without `--format`, `--ppi`, and the page placeholder.

## Result

Both platforms produced the same SHA-256 for every file.

| File | SHA-256 |
|---|---|
| `complete-1.png` | `4c2933f27f9775ebbd683993258766c5d13a45930ac50b2e4686c8324cc45949` |
| `complete-2.png` | `55c02bfa8ad23c60b51fdd086419a760168f3e96536956fbeece4d375adbfa82` |
| `custom-running-elements-1.png` | `2f626860f45084bf359587a804927041f7cc2b4f867387c56ae832010f037381` |
| `custom-running-elements-2.png` | `d06543a789b61ddbaf040a104c4ab6713eaf5a361807741fede8f6cdb7295053` |
| `minimal-1.png` | `d6ab2942f9fb38fe36e4ddd8e9d171cd77ac496dd200a0a8f76d643d88d3bc0d` |
| `complete.pdf` | `aa3cb6ee7eb426027a9c48da5682c38bfcdf9bf65e736a747beb4fb5a0f69110` |

Two further observations:

- The five PNG digests equal the golden digests that the original recorded
  under Typst 0.15.0. For these pages, 0.15.1 renders the same bytes.
- Typst 0.15.1 installed natively on macOS through Homebrew produced the same
  six digests.

## Decision

Golden pages serve both architectures. The test image is built for the host
platform, and no target pins `--platform`.

The gate still runs in the Linux image, because that is the environment
production uses. A native run gives the same bytes today and is good for
fast feedback, but it does not close a gate.

## When to measure again

- When the pinned Typst version changes.
- When a golden page fails on one machine and passes on another.

## Note of October 2026: new faces, one architecture measured

The capacity report changed its faces in 2026-10: Public Sans 2.001 for
words and JetBrains Mono 2.304 for names, numbers, and status glyphs,
beside Liberation Sans, which the generic layer keeps as its default. The
golden pages were rendered again, and on linux/amd64 only. Rendering on
linux/arm64 and on macOS is to be measured again before the next release;
until then the equality above is known for the examples of the generic layer
alone.
