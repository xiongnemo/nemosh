# gdamore/encoding, as nemosh carries it

This is [github.com/gdamore/encoding](https://github.com/gdamore/encoding) v1.0.1
(commit `6770ff7f5dae`, the last upstream commit, 2024-03-14), which tcell imports and
nemosh therefore links. The root `go.mod` replaces the module with this directory.

**Why.** Upstream builds its four character sets -- ASCII, ISO8859-1, ISO8859-9 and
EBCDIC -- in an `init` each: a map and 256 small slices apiece, 1,103 allocations at the
start of every program that links the package, used or not. That was 40% of nemosh's
package init, paid by every script, `nemosh -c` and applet shim, for tables tcell
consults only on a Unix terminal with a legacy locale.

**What changed.** The tables are data now, generated into `tables.go` and laid down by
the linker, so init does nothing. Nothing else changed: the API is upstream's, a
`Charmap` built at run time from a `Map` is initialized by upstream's algorithm as
before, and upstream's tests pass unmodified. Each modified file says so in its header.

- `charmap.go`: a `Charmap` may carry a generated table (`encodes`, and `runes` filled
  in); `Init` leaves one alone, and the encoder looks a rune up in it by binary search.
- `ascii.go`, `latin1.go`, `latin5.go`, `ebcdic.go`: each variable points at its
  generated table instead of being built in `init`.
- `charmap_source_test.go`: the four maps, moved verbatim from those files.
- `tables_test.go`: the generator, and what holds it to upstream. `tables.go` has to be
  exactly what the maps generate, and every byte and every rune has to decode and encode
  as upstream's algorithm makes them.
- `go.mod`: `go 1.26` and nemosh's `golang.org/x/text`, so the module tests with the
  versions nemosh builds with.

**Maintaining it.** Upstream has not moved since 2024. If it does, copy the new release
over this directory, put the four maps into `charmap_source_test.go` again, reapply the
`charmap.go` change, and regenerate:

```sh
go -C third_party/gdamore-encoding test -run TestTables_areWhatTheSourceMapsGenerate -update .
go -C third_party/gdamore-encoding test ./...
```

Licensed under the Apache License 2.0, as upstream is; see `LICENSE`.

---
## encoding


[![Linux](https://img.shields.io/github/actions/workflow/status/gdamore/encoding/linux.yml?branch=main&logoColor=grey&logo=linux&label=)](https://github.com/gdamore/encoding/actions/workflows/linux.yml)
[![Windows](https://img.shields.io/github/actions/workflow/status/gdamore/encoding/windows.yml?branch=main&logoColor=grey&logo=windows&label=)](https://github.com/gdamore/encoding/actions/workflows/windows.yml)
[![Apache License](https://img.shields.io/github/license/gdamore/encoding.svg?logoColor=silver&logo=opensourceinitiative&color=blue&label=)](https://github.com/gdamore/encoding/blob/master/LICENSE)
[![Coverage](https://img.shields.io/codecov/c/github/gdamore/encoding?logoColor=grey&logo=codecov&label=)](https://codecov.io/gh/gdamore/encoding)
[![GoDoc](https://img.shields.io/badge/godoc-reference-blue.svg)](https://godoc.org/github.com/gdamore/encoding)

Package encoding provides a number of encodings that are missing from the
standard Go [encoding]("https://godoc.org/golang.org/x/text/encoding") package.

We hope that we can contribute these to the standard Go library someday.  It
turns out that some of these are useful for dealing with I/O streams coming
from non-UTF friendly sources.

The UTF8 Encoder is also useful for situations where valid UTF-8 might be
carried in streams that contain non-valid UTF; in particular I use it for
helping me cope with terminals that embed escape sequences in otherwise
valid UTF-8.
