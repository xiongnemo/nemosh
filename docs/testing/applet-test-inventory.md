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
| `wc` | POSIX | `-l`, `-w`, `-c`, stdin and files, [multibyte policy](#wc-multibyte-policy). |
| `basename` | POSIX | simple paths, trailing slashes, suffix removal. |
| `dirname` | POSIX | simple paths, root paths, no-slash paths, Windows forward paths. |
| `ls` | POSIX/BusyBox + Windows | files/dirs, `-a`, `-l` if supported, Unicode names, case-aware glob inputs, UNC share root. |
| `mkdir` | POSIX | create dir, `-p`, existing dir, parent missing, Windows path aliases. |
| `rmdir` | POSIX | remove empty dir, non-empty failure, missing dir. |
| `rm` | POSIX/common | remove file, `-r`, `-f`, missing path, readonly/ACL behavior documented. |
| `touch` | POSIX | create file, update mtime, `-c`, path aliases, permission failure. |

### `wc` Multibyte Policy

- `wc -c` counts raw input bytes, not Unicode code points.
- `wc -w` incrementally decodes UTF-8 and counts maximal runs of non-whitespace Unicode code points, using Unicode whitespace classification. Input chunk boundaries do not affect line, word, or byte counts.
- Invalid UTF-8 consumes one input byte as a replacement rune. That byte contributes to `-c`, and the replacement rune is non-whitespace for `-w`.

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

Written before v0 as a list of what should not block the first runtime. **Almost all
of it has since landed**, so it is kept here as the record of what each group's test
focus turned out to be -- the "initial test focus" column was a prediction, and
where it was right it is worth saying so.

| Group | Applets | Test focus, as it turned out | |
| --- | --- | --- | --- |
| Checksums | `cksum`, `md5sum`, `sha1sum`, `sha256sum`, `sha384sum`, `sha512sum`, `sha3sum`, `crc32`, `sum` | known vectors, stdin/file, binary mode -- as predicted. 58 of 58 measured forms agree with busybox. | done |
| Archiving | `tar`, `unzip`, `cpio`, `ar`, `gzip`, `gunzip`, `zcat`, `bunzip2`, `bzcat` | **path traversal safety was the right call and became the largest test file in the group**: one containment helper, fifteen hostile names, each asserting nothing was written outside the root. | done |
| Text transforms | `cut`, `tr`, `sort`, `uniq`, `comm`, `paste`, `join`, `expand`, `unexpand`, `fold`, `tsort`, `shuf`, `strings`, `factor`, `base32`, `ascii`, `od`, `hexdump`, `hd`, `diff`, `patch`, `dos2unix`, `unix2dos`, `iconv` | POSIX option subsets, yes; the UTF-8 policy turned out to need an applet of its own, and `iconv` is where it now lives. | done |
| File inspection | `stat`, `readlink`, `realpath`, `du` | Windows path roots and reparse points, as predicted. `df` is not implemented. | mostly |
| Process | `ps`, `kill`, `sleep`, `timeout`, `yes`, `pgrep`, `pkill`, `top`, `free` | the Windows process model and the Ctrl-C limits, as predicted. | done |
| Networking | `wget`, `nc`, `whois`, `ssl_client`, `httpd`, `ftpget`, `ftpput` | not "post-v0 unless needed for scripts" in the end. The focus is containment -- a URL and a request path are untrusted names, so both go through the *archive* helper above -- and every test runs against a server the test starts. | done |
| Editors | `nano`, `micro` | one implementation under two names, keyed by `argv[0]`, on the reading that busybox's own `vi` is a from-scratch clone. Headless tests over a tcell simulation screen. | done |
| Interpreters | `vi`, `ed`, `awk`, `bc`, `dc` | still substantial standalone projects, and still deferred. This is the one row that has not moved. | deferred |
| No Go support | `xz`, `unxz`, `lzma`, `lzop`, `bzip2` (compressing) | **not implemented and deliberately not registered**, so PATH still finds a real one. The reasons are in `docs/support-matrix.md`. | out |

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
