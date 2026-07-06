# Applet Test Inventory

Every Nemosh applet must have behavior tests before it is considered part of a
release milestone. This follows BusyBox testsuite's per-applet discipline while
using Nemosh's metadata format.

## Test Requirement Levels

- Smoke: applet starts, parses basic options, and handles stdin/stdout/status.
- POSIX: behavior required by POSIX is covered for supported options.
- BusyBox: behavior follows BusyBox/busybox-w32 for non-POSIX choices.
- Windows: native Windows path, ACL, CRLF, Unicode, or device behavior is covered.
- Negative: invalid options, missing operands, permission errors, and missing files
  produce expected status and diagnostics.

No applet should be marked done without at least Smoke and Negative tests.

## Milestone A: Shell-Critical Applets

| Applet | Semantics | Required tests |
| --- | --- | --- |
| `true` | POSIX | exits 0; ignores operands according to selected behavior. |
| `false` | POSIX | exits non-zero; ignores operands according to selected behavior. |
| `echo` | POSIX/BusyBox | basic args, spacing, newline; avoid ambiguous `-e`/`-n` claims until defined. |
| `printf` | POSIX | `%s`, `%d`, width basics, escapes, missing/reused operands, no implicit newline. |
| `pwd` | POSIX + busybox-w32 path | logical cwd, Windows current-root behavior, UNC cwd, opportunistic case display. |
| `env` | POSIX + Windows env | print env, `-i`, `NAME=VALUE command`, empty values, case-collision spawn behavior. |
| `printenv` | BusyBox/common | all vars, named var, missing var status. |
| `test` | POSIX | string tests, integer tests, file tests, operator precedence subset, errors. |
| `[` | POSIX | same as `test`, plus required closing bracket diagnostics. |

## Milestone B: Core File/Text Applets

| Applet | Semantics | Required tests |
| --- | --- | --- |
| `cat` | POSIX | stdin, one file, multiple files, missing file, binary data, `/dev/null`. |
| `head` | POSIX/common | default 10 lines, `-n`, stdin, short files, missing file. |
| `tail` | POSIX/common | default 10 lines, `-n`, stdin, short files, follow deferred unless implemented. |
| `wc` | POSIX | `-l`, `-w`, `-c`, stdin and files, multibyte policy documented. |
| `basename` | POSIX | simple paths, trailing slashes, suffix removal. |
| `dirname` | POSIX | simple paths, root paths, no-slash paths, Windows forward paths. |
| `ls` | POSIX/BusyBox + Windows | files/dirs, `-a`, `-l` if supported, Unicode names, case-aware glob inputs, UNC share root. |
| `mkdir` | POSIX | create dir, `-p`, existing dir, parent missing, Windows path aliases. |
| `rmdir` | POSIX | remove empty dir, non-empty failure, missing dir. |
| `rm` | POSIX/common | remove file, `-r`, `-f`, missing path, readonly/ACL behavior documented. |
| `touch` | POSIX | create file, update mtime, `-c`, path aliases, permission failure. |

## Milestone C: Core Mutation/Search Applets

| Applet | Semantics | Required tests |
| --- | --- | --- |
| `cp` | POSIX/common | file copy, overwrite, directory copy if `-r`, metadata policy, symlink/reparse behavior. |
| `mv` | POSIX/common | rename, overwrite, cross-volume fallback if implemented, directory move. |
| `chmod` | POSIX-facing + Windows ACL | simple mode bits if exposed, readonly bit/ACL mapping, unsupported mode diagnostics. |
| `grep` | POSIX | literal/regex basics, `-i`, `-v`, `-n`, stdin/files, binary policy. |
| `sed` | POSIX subset | `s///`, `-n`, `p`, `d`, file/stdin, CRLF input behavior. |
| `find` | POSIX subset | path traversal, `-name`, `-type`, `-print`, Windows symlink/junction policy. |
| `xargs` | POSIX | whitespace splitting, `-0` if supported, command status propagation. |

## Milestone D: Windows-Native Extension Applets

| Applet | Semantics | Required tests |
| --- | --- | --- |
| `winpath` | Nemosh | `/c/foo` to `C:\foo` or `C:/foo`, UNC conversion, invalid paths. |
| `posixpath` | Nemosh | `C:\foo`/`C:/foo` to `/c/foo`, UNC to `//host/share`, case policy. |
| `shares` | Nemosh/Windows | explicit `//host` share enumeration, disabled/config-gated behavior, network errors. |
| `nmount` | Nemosh extension | post-v0 virtual mount registration, listing, removal, path resolution effects. |
| `acl` or `nacl` | Windows ACL | inspect owner/ACE summary, permission denied, Unicode paths. Name still open. |

## Later BusyBox-Style Roadmap

These applets are useful but should not block the first runtime unless tests or
scripts require them:

| Group | Applets | Initial test focus |
| --- | --- | --- |
| Checksums | `cksum`, `md5sum`, `sha1sum`, `sha256sum`, `sha512sum` | known vectors, stdin/file, binary mode. |
| Archiving | `tar`, `gzip`, `gunzip`, `bzip2`, `bunzip2`, `xz`, `unxz`, `zcat` | simple archives, stdin/stdout, path traversal safety. |
| Text transforms | `cut`, `tr`, `sort`, `uniq`, `comm`, `paste`, `join`, `expand`, `unexpand` | POSIX option subsets, locale/UTF-8 policy. |
| File inspection | `stat`, `readlink`, `realpath`, `du`, `df` | Windows path roots, symlink/junction/reparse points, volume mounts. |
| Process | `ps`, `kill`, `sleep`, `timeout`, `yes` | Windows process model, Ctrl-C/TERM limitations. |
| Networking | `wget`, `nc`, `ftpget`, `ftpput` | post-v0 unless needed for scripts. |
| Editors/calculators | `vi`, `ed`, `awk`, `bc`, `dc` | substantial standalone projects; defer unless explicitly prioritized. |

## Per-Applet Test File Rule

Each implemented applet gets at least one file:

```text
tests/behavior/applets/<applet>.toml
```

Large applets may use a directory:

```text
tests/behavior/applets/grep/basic.toml
tests/behavior/applets/grep/errors.toml
tests/behavior/applets/grep/windows.toml
```

The implementation checklist for an applet is:

- [ ] Smoke tests exist.
- [ ] Negative tests exist.
- [ ] POSIX-required supported options are tested.
- [ ] Windows path behavior is tested if the applet opens files.
- [ ] Binary/text behavior is documented and tested.
- [ ] Reference differences are recorded.
