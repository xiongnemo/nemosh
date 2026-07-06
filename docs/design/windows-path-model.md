# Windows Path Model

This document records the initial Windows path model for Nemosh. It is a design
draft, not final implementation code.

## Goals

- Preserve POSIX shell semantics on Windows as much as practical.
- Accept common Windows path forms without making Windows paths the internal
  shell model.
- Use busybox-w32 as the primary native Windows behavior reference.
- Treat MSYS2 and Cygwin as non-target references only: useful for studying
  edge cases, not compatibility targets for Nemosh behavior.

## Confirmed Direction

| Topic | Decision |
| --- | --- |
| Internal model | Accept both POSIX-like and Windows path input, but normalize shell-internal paths to POSIX-like form. |
| Primary native reference | Use busybox-w32 as the main native Windows reference. |
| Drive mapping | Support `/c/foo` as Nemosh's short drive alias. This is not an MSYS2 compatibility promise. |
| Mount-prefix mapping | Support `/mnt/c/foo` by default as a configurable mount-prefix alias. It is semantically distinct from `/c/foo` and is not a WSL compatibility promise. |
| Default display | Use `posix-drive`: shell-generated paths such as `pwd`, diagnostics, and completion candidates should prefer `/c/foo`. |
| `/tmp` | Map `/tmp` to `%TEMP%` or `%TMP%`. |
| UNC | Follow busybox-w32/Cygwin-compatible forward-slash UNC form: `//host/share/path`. |
| Sample config | Final sample config and docs must comment each path scheme separately; do not collapse `/c` and `/mnt/c` into one generic alias knob. |
| Current root | Follow busybox-w32: `/foo` is relative to the current root, where the root can be a drive such as `D:/` or a UNC share such as `//host/share`. |
| Path case | Follow busybox-w32's pragmatic approach for shell-generated display paths: try to resolve real filesystem case, silently preserve spelling on failure. Keep virtual drive aliases lowercase, e.g. `/c`. |
| Virtual roots | `/tmp`, `/dev`, and similar Nemosh virtual roots are enabled by default and resolved before current-root expansion. They must be configurable. |
| Mount table | Do not implement a Nemosh-owned mount table in v0. Follow busybox-w32 by using Windows-provided roots, drive letters, UNC shares, volume mount points, junctions, mapped/subst drives, and configured path aliases. |
| Links/reparse points | Mimic busybox-w32 where practical: recognize and use Windows-native symlinks, junctions, and reparse points; do not invent a POSIX symlink facade or treat `.lnk` shortcuts as shell symlinks in v0. |
| Backslash syntax | Preserve POSIX shell lexer semantics: unquoted backslash is an escape. Windows paths should use forward slashes such as `C:/Users/nemo` or be quoted. |

## Path Forms

Nemosh should recognize these Windows-local paths:

```text
/c/Users/nemo/file.txt        primary POSIX drive alias
/mnt/c/Users/nemo/file.txt    mount-prefix alias
C:/Users/nemo/file.txt        accepted Windows path input
C:\Users\nemo\file.txt        accepted Windows path input
//server/share/dir/file.txt   UNC path, forward-slash form
```

The shell should prefer POSIX-like display and script-visible paths:

```text
pwd -> /c/Users/nemo/project
```

Like busybox-w32, Nemosh should model Windows roots explicitly. A root can be a
drive or a UNC share. A leading single slash is relative to the current root,
not to a global Unix filesystem root:

```text
cd /c/Users/nemo       # current root is drive C:
cd /                   # goes to /c

cd //server/share/dir  # current root is //server/share
cd /                   # goes to //server/share
cd /tmp                # means //server/share/tmp while that UNC root is current
```

Virtual roots such as `/tmp` and `/dev` must be resolved before current-root
expansion so they remain globally meaningful Nemosh paths.

UNC host-only paths are not valid roots. Following busybox-w32 and Win32 UNC
semantics, `//host/share` is a root, while `//host` is not:

```text
cd //192.168.1.13/f   # valid if share f exists
cd //192.168.1.13     # invalid: host-only UNC is not a filesystem root
```

When `cd` receives a host-only UNC path, it should return a targeted diagnostic
instead of a generic path failure, for example: `//host is not a directory root;
use //host/share`.

This is not primarily a shell limitation. In Win32 filesystem terms, the
directory root is `\\host\share`; `\\host` is a network resource enumeration
entry, not a normal filesystem directory that can be used as the current working
directory. Nemosh can later provide a higher-level applet or virtual browser for
host shares, for example `shares //host` or optional `ls //host`, but `cd //host`
should not silently become a filesystem cwd unless a separate virtual namespace
design explicitly chooses that behavior.

If Nemosh later supports host-level share browsing, it should be documented as a
Nemosh extension layered above Win32 network enumeration APIs, not as BusyBox ash
or POSIX path semantics.

## Windows Mounts And Volumes

Nemosh v0 should not provide its own `mount` table or WSL-like namespace. Follow
busybox-w32's approach: use the roots and mount points Windows already exposes.

Supported by path resolution, subject to Windows itself accepting the path:

- Drive roots: `C:/`, `D:/`, and Nemosh display aliases such as `/c`, `/d`.
- Mapped or `subst` drives exposed through drive letters, such as `F:/` and
  `/f`.
- UNC share roots such as `//host/share`.
- Existing Windows volume mount points and junctions inside the filesystem.

`/mnt/c` remains a configurable syntactic alias for `/c`, not evidence of a
general WSL-style mount namespace. User-defined mounts such as
`mount --bind //server/share /media` are post-v0 extension territory and require
a separate virtual namespace design.

Host-level network browsing should use an explicit applet rather than implicit
`cd //host` magic. A future applet can enumerate shares and mount selected
network resources into a Nemosh virtual namespace, for example:

```sh
shares //server
nmount //server/share /net/share
```

This keeps `//host/share` as the native UNC filesystem root while making
host-level browsing and virtual mounts explicit Nemosh extensions. `cd //host`
should still diagnose that `//host` is not a filesystem directory root unless a
future virtual namespace mode deliberately changes the current-directory model.

Windows spelling should be used at platform boundaries when the target API or
native process requires it.

## External Argv Policy

Nemosh must not perform general path auto-conversion on ordinary argv elements
passed to external native programs. This follows the busybox-w32 direction more
closely than MSYS2-style argv rewriting and avoids corrupting regexes, URLs,
Git refspecs, language snippets, and other strings that merely look path-like.

Path conversion is allowed only where Nemosh owns the semantics:

- Shell operations such as `cd`, redirection, globbing, `.`/`source`, and script
  file loading.
- Bundled applets that use the Nemosh path layer.
- Executable lookup and the `argv[0]` process-spawn boundary when Windows APIs
  require native spelling.
- Explicit helper commands such as `winpath` and `posixpath`.

Ordinary external program arguments are passed through unchanged. Users can pass
native paths explicitly:

```sh
some.exe C:/tmp/a.txt
some.exe "$(winpath /c/tmp/a.txt)"
```

## Reference Behavior Notes

- busybox-w32 recommends forward slashes and documents absolute Windows paths as
  `c:/path` and UNC paths as `//host/share/path`.
- MSYS2 uses `/c/foo` in documented `cygpath -u C:\foo` examples and performs
  automatic path conversion for native executables, but Nemosh should not treat
  this as a target behavior.
- Cygwin defaults to `/cygdrive/c`, but its cygdrive prefix is configurable and
  UNC paths using forward slashes are supported as `//machine/share/...`.
  Nemosh may study these choices without adopting Cygwin compatibility.

## Initial Normalization Rules

| Input | Internal canonical candidate | Notes |
| --- | --- | --- |
| `C:/a/b` | `/c/a/b` | Drive is lowercased in the virtual root only. |
| `C:\a\b` | `/c/a/b` | Backslashes are accepted at input boundaries. |
| `/c/a/b` | `/c/a/b` | Nemosh short drive alias. |
| `/mnt/c/a/b` | `/c/a/b` | Mount-prefix alias normalized to the internal drive form. |
| `//host/share/a` | `//host/share/a` | UNC stays in double-slash POSIX form. |
| `/tmp/a` | `/tmp/a` | Virtual mount backed by `%TEMP%` or `%TMP%`. |

When the current root is a drive, `/a/b` resolves under that drive root. When the
current root is a UNC share, `/a/b` resolves under that share root. This matches
busybox-w32 behavior observed with `ash`: `cd //192.168.1.200/Media/navidrome;
cd /; pwd` prints `//192.168.1.200/Media`.

`/cygdrive/c/a/b` should be implemented as an optional, explicitly enabled
Cygwin-style prefix with the default set to off. It is not a v0 compatibility
goal.

## Configuration Shape

The final sample configuration must document each path scheme separately because
the schemes have different semantics. Avoid a vague setting such as
`path.aliases=c,mnt`.

Illustrative shape, not final syntax:

```sh
# Display shell-generated Windows paths as /c/foo rather than C:/foo.
nemosh path display posix-drive

# Accept /c/foo as Nemosh's short drive alias for C:\foo.
nemosh path accept drive-short on

# Accept /mnt/c/foo as a mount-prefix alias. This is distinct from /c/foo.
nemosh path accept mount-prefix /mnt

# Accept native Windows drive paths written with forward slashes, like C:/foo.
nemosh path accept windows-forward on

# Accept UNC shares in forward-slash form, like //host/share/foo.
nemosh path accept unc on

# Do not accept /cygdrive/c/foo by default; Cygwin compatibility is not a goal.
nemosh path accept cygdrive off
```

## Device Paths

busybox-w32 provides Unix-style device files such as `/dev/null`, `/dev/tty`,
`/dev/zero`, and `/dev/urandom`. Nemosh should support a practical v0 set for
shell redirection, bundled applets, random bytes, and text clipboard access.

Candidate device paths:

```text
/dev/null
/dev/stdin
/dev/stdout
/dev/stderr
/dev/fd/N
/dev/tty
/dev/zero
/dev/urandom
/dev/random
/dev/clipboard
```

v0 required:

- `/dev/null`
- `/dev/stdin`
- `/dev/stdout`
- `/dev/stderr`
- `/dev/zero`
- `/dev/urandom`
- `/dev/random`, implemented as the same non-blocking CSPRNG source as
  `/dev/urandom`; do not promise Linux entropy-blocking semantics.
- `/dev/clipboard`, text-only, UTF-8 at the shell/applet boundary.

`/dev/fd/N` should be supported once the shell-owned fd table is stable. It maps
to Nemosh fd entries, not to a Windows filesystem path, and is only promised for
shell operations, builtins, and bundled applets.

TTY/PTY paths and semantics are deferred. `/dev/tty`, `/dev/ptmx`, and
`/dev/pts/*` are not v0 compatibility gates because native Windows terminal,
ConPTY, and job-control behavior need a separate design.
