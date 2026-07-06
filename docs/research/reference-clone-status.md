# Reference Clone Status

References are shallow-cloned under `references/`, which is intentionally ignored
by Git. Use this document to track what is locally available for research.

## Current Local Clones

| Reference | Path | Shallow | HEAD | Approx. size | Notes |
| --- | --- | --- | --- | ---: | --- |
| zsh | `references/shells/zsh` | yes | `4b4ebcc` | 18.1 MB | Long-term interactive and zsh capability reference. |
| bash | `references/shells/bash` | yes | `b460816` | 46.2 MB | Real-world script and POSIX mode comparison. |
| dash | `references/shells/dash` | yes | `8a602ac` | 0.9 MB | Small POSIX shell baseline. |
| mksh | `references/shells/mksh` | yes | `3417528` | 2.1 MB | Alternate ksh/POSIX behavior reference. |
| yash | `references/shells/yash` | yes | `1e7b308` | 5.3 MB | Strict POSIX-oriented behavior reference. |
| BusyBox | `references/shells/busybox` | yes | `a448b6d` | 14.1 MB | BusyBox ash and compact userland reference. |
| Oils | `references/shells/oils` | yes | `15de8fd` | 82.9 MB | Spec tests and shell semantic modeling reference. |
| mvdan/sh | `references/go-shells/mvdan-sh` | yes | `2255122` | 1.3 MB | Go parser/AST/interpreter reference. |
| MSYS2 runtime | `references/windows-compat/msys2-runtime` | yes | `01d6c70` | 79.5 MB | Windows POSIX compatibility layer reference. |
| newlib-cygwin | `references/windows-compat/newlib-cygwin` | yes | `41e6325` | 81.5 MB | Cygwin runtime and POSIX emulation reference. |
| busybox-w32 | `references/windows-compat/busybox-w32` | yes | `b5c1253` | 16.1 MB | Native Win32 BusyBox/ash reference. |

Total local reference size is approximately 348 MB as of this snapshot.

## BusyBox MinGW Note

The requested BusyBox MinGW/native Windows reference appears to be represented by
`rmyorston/busybox-w32`. The upstream page describes busybox-w32 as a Win32 API
port built with MinGW-w64 and llvm-mingw, with source mirrored on GitHub at
`https://github.com/rmyorston/busybox-w32`.

## Next Use

- Build the first behavior matrix from these local references.
- Record source file paths and tests consulted for each behavior claim.
- If a behavior requires history archaeology, deepen only that specific clone.
