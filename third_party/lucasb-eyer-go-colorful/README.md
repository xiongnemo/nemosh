# lucasb-eyer/go-colorful, as nemosh carries it

This is [github.com/lucasb-eyer/go-colorful](https://github.com/lucasb-eyer/go-colorful)
v1.3.0, which tcell imports and nemosh therefore links. The root `go.mod` replaces the
module with this directory.

**Why.** Upstream's `hexcolor.go` makes a `HexColor` that is a `database/sql` Scanner and
Valuer and a JSON and YAML marshaller. tcell uses none of that, and nemosh nothing of
go-colorful but what tcell does, yet the file is in the package, so every build linked
`database/sql/driver` and `encoding/json` for it. Under Go 1.27 `database/sql/driver` imports
`uuid`, which imports `crypto/rand`.

**What changed.** Three files are left out, and nothing else is touched:

- `hexcolor.go` and `hexcolor_test.go`: the `HexColor` type and its test.
- `hsluv_test.go` and `hsluv-snapshot-rev4.json`: the test that checks HSLuv against a
  1.6 MB snapshot, which is not worth carrying for code tcell does not call.
- `go.mod`: `go 1.26`, so the module tests with the language nemosh builds with.

The README, CHANGELOG and `.gitignore` are upstream's documentation and are not copied;
`LICENSE` is.

**Maintaining it.** Copy a new release over this directory, delete the files above again,
and run:

```sh
go -C third_party/lucasb-eyer-go-colorful vet ./...
go -C third_party/lucasb-eyer-go-colorful test ./...
```

`cmd/nemosh/release_deps_test.go` fails if a release build links `database/sql/driver`
again.

Licensed under the MIT License, as upstream is; see `LICENSE`.
