# ulikunitz/xz, as nemosh carries it

This is [github.com/ulikunitz/xz](https://github.com/ulikunitz/xz) v0.5.17, which nemosh's
unxz, xzcat, unlzma and lzcat read through and tar's -J and --lzma write through. The root
`go.mod` replaces the module with this directory.

**Why.** `format.go` imports `crypto/sha256` for the one check method that is SHA-256, and
`crypto/sha256` brings Go's FIPS module, whose entropy source commits a 32 MiB buffer in every
process that links it, whether or not a hash is ever asked for. nemosh's other hashes are the
system's, CNG's (`internal/digest`), so this was the last thing linking it.

**What changed.** One file is changed and two added; nothing else is touched:

- `format.go`: `newHashFunc` takes SHA-256 from an exported `SHA256New func() hash.Hash`
  rather than from `crypto/sha256`, and refuses a stream checked with SHA-256 when it is not
  set (`errNoSHA256`), rather than reading it unchecked. nemosh sets it to `digest.SHA256` in
  `internal/applets/compress_xz.go`.
- `sha256_test.go`: sets `SHA256New` to `crypto/sha256`'s for upstream's tests, which a test
  binary may link.
- `go.mod`: `go 1.26`, so the module tests with the language nemosh builds with.
- `.gitattributes`: `lzma/examples` byte for byte, since `a.txt` and `info.txt` have CRLF line
  endings, `a.lzma` decompresses to exactly those bytes, and the repository's `eol=lf` would
  make the tests that compare them fail on a fresh checkout.

Only the packages a nemosh build links are carried -- the root, `lzma`, `internal/hash` and
`internal/xlog` -- and `internal/randtxt` and `lzma/examples`, which their tests use. Left out: `cmd/`, `xio` and the
packages only it uses, `testdata/enwik7`, which only the benchmarks read (fetch it from upstream
to run them), and upstream's documentation and CI. `LICENSE` is copied.

**The check is still made.** busybox's unxz does not verify an xz stream's SHA-256, and this
does, as it did before the fork; a stream whose check does not match is `corrupted data`.

**Maintaining it.** Copy a new release's root, `lzma` with its `examples`, `internal/hash`,
`internal/xlog` and `internal/randtxt` over this directory, make the change to `format.go`
again, and run:

```sh
go -C third_party/ulikunitz-xz vet ./...
go -C third_party/ulikunitz-xz test ./...
```

`cmd/nemosh/release_deps_test.go` fails if a release build links `crypto/sha256` again.

Licensed under the BSD 3-Clause License, as upstream is; see `LICENSE`.
