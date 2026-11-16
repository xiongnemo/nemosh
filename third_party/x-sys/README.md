# golang.org/x/sys, as nemosh carries it

This is [golang.org/x/sys](https://pkg.go.dev/golang.org/x/sys) v0.47.0, which nemosh calls
Windows through and tcell and x/term import. The root `go.mod` replaces the module with this
directory.

**Why.** `windows/types_windows.go` imports `net`, for one method, `SocketAddress.IP`, which
returns a `net.IP`. Importing `net` runs its init, which on Windows starts Winsock: 0.5 to 1.6
ms of every start and 18 allocations, measured (`GODEBUG=inittrace=1`), in a shell that opens
a socket only for nc, wget and the rest --
and those speak Winsock directly now (`internal/netio`). Nothing in nemosh, tcell or x/term
calls `IP`.

**What changed.** In `windows/types_windows.go`, the `IP` method and the `net` import are left
out, with a comment where they were. Nothing else in the code is touched.

**What is carried.** The production files of `windows`, `unix` and `plan9`, and upstream's
`go.mod`, `LICENSE` and `PATENTS`. nemosh builds for Windows alone, but `go mod tidy` resolves
every platform's imports, and x/term and tcell import `unix` and `plan9` on theirs, so those
two have to be here for the module graph even though no nemosh build compiles them. Left out:
upstream's tests and their testdata, which depend on the machine they run on (one lists
WireGuard drivers, and is refused without the rights to); `windows/registry`, `windows/svc`
and `windows/mkwinsyscall`, which nothing here imports; `cpu`, `execabs` and the generators
under `unix/`.

**`vet.flags`.** The loop over `third_party/*/` in product.yml and AGENTS.md passes a module's
`vet.flags` to `go vet`. Here it is `-unsafeptr=false`: upstream's generated syscall wrappers
turn the uintptr a system call returns into a pointer, which is what that analyzer reports and
what a syscall package has to do. Every other analyzer runs.

**Maintaining it.** Copy a new release's production `.go` and `.s` files of `windows`, `unix`
and `plan9` over this directory, leave out `IP` and the `net` import again, and run:

```sh
go -C third_party/x-sys vet -unsafeptr=false ./...
```

`cmd/nemosh/release_deps_test.go` fails if a release build links `net` again, and
`GODEBUG=inittrace=1 nemosh --version` should print no `init net`.

Licensed under the BSD 3-Clause License, as upstream is; see `LICENSE` and `PATENTS`.
