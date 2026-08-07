# Support Matrix

Everything here was **measured against a built binary on 2026-08-07**, not read
off the source or carried from an earlier document. The probe ran each applet
with an option it could not possibly implement (`-%`), then with each plausible
option letter, and recorded what came back. Where this table says an option is
unsupported, that is an observation.

Re-measure this file rather than editing it by hand when applet coverage
changes; see AGENTS.md, Documentation Hygiene.

## Platforms

| Platform | Status | What that means |
| --- | --- | --- |
| `windows/amd64` | **Supported** | The target. Bugs here are bugs. Behavior corpus, differential suite against busybox-w32/ash, and native path, launch, device, and interrupt tests all run here. |
| `linux/amd64` | **Build and test only** | CI compiles and runs the full suite, which is what keeps the platform splits from rotting. Not a support commitment: three interactive interrupt tests are skipped here, and the Windows-only surfaces (clipboard device, `ComSpec` batch launch, 8.3 fallback, case-preserving `cd`) have no counterpart. |
| `darwin/*` | **Compile check only** | `GOOS=darwin go build ./...` runs in CI. Nothing is executed. |
| `windows/arm64` | **Untested** | Not built, not run, not claimed. |

Go 1.26, `CGO_ENABLED=0`, single binary, no runtime sidecars.

## Shell

Implemented and covered by the behavior corpus (143 cases) and the differential
suite: sequential lists, pipelines with `!` negation, `&&`/`||`, brace groups,
subshells, functions, `if`/`elif`, `for`, `while`/`until`, `case` including the
one-line forms, heredocs, redirections including `>|` and `<>`, background jobs
with `jobs` and `wait`, traps, parameter expansion with the selected operators,
field splitting, pathname expansion, arithmetic expansion, command substitution
in both spellings, aliases, and `local`.

### Refused on purpose, with a reason and a non-zero status

A capability that is absent fails loudly rather than approximating. Each of
these names why, and names what busybox-w32 does with the same name.

| Name | Status | Why |
| --- | --- | --- |
| `hash` | 126 | Command lookup is not cached, so there is nothing to remember or forget. busybox-w32 does implement it, over a hash table this shell does not have. |
| `ulimit` | 126 | Windows has no `getrlimit`. busybox-w32 does not implement it either — it keeps the name and returns 1 with no message. |
| `fg`, `bg` | 126 | Job control needs a terminal process group, which Windows does not have. busybox-w32 compiles both out under `#if JOBS`. |
| `set -b` | 2 | Asynchronous completion is reported when `wait` or `jobs` asks; there is no notification channel to switch on. |
| `set -n`, `set -v` | 2 | A script is parsed in full before any of it runs, so by the time the option is set there is no unread input left to withhold or echo. |

### Known divergences from bash/dash/ash

- **Parse before effects.** A syntax error anywhere in a script means none of it
  runs. bash and dash execute up to the error. `{echo bad;}` produces no output
  here; both references print nothing either but reach the command first.
- `~user` is left as written. `~` and `~/path` work.
- An alias whose value is not a list of words is refused at definition time,
  because substitution happens after parsing.
- `${#@}` is not pinned; POSIX leaves it unspecified and the references disagree.

## Applets

All 39 registered applets ship. **Name presence is not option parity**, and the
column that matters is the third one.

| Applet | Options implemented | Unknown option is |
| --- | --- | --- |
| `basename` | none; the `basename PATH [SUFFIX]` form works | refused by name |
| `cat` | none | **misleading** — reported as a missing file |
| `chmod` | numeric mode | refused by name |
| `cp` | none | refused by name |
| `cut` | `-b -c -d -f -n -s` | refused by name |
| `date` | `-d -u` | refused by name |
| `dirname` | none needed | refused by name |
| `echo` | `-n -e` | treated as text, which is what `echo` does |
| `env` | `-i`, and `NAME=VALUE command` | refused by name |
| `find` | **none** | **silently wrong — see below** |
| `grep` | `-i -n -v` | refused by name |
| `head` | `-n` | **misleading** — reported as a missing file |
| `ln` | `-s` | refused by name |
| `ls` | `-a -h -l` | refused by name |
| `mkdir` | `-m -p -v` | refused by name |
| `mv` | none | refused by name |
| `posixpath` | none | treated as a path operand |
| `printenv` | none | treated as a variable name |
| `printf` | format string | treated as the format, which is correct |
| `pwd` | `-L -P` both accepted | accepted |
| `readlink` | `-n` | refused by name |
| `realpath` | none | treated as a path operand |
| `rm` | `-f -r` | refused by name |
| `rmdir` | `-p -v` | refused by name |
| `sed` | `s///` substitution | refused by name |
| `sleep` | duration operand | reported as an invalid duration |
| `sort` | `-n -r` | refused by name |
| `tail` | `-n` | **misleading** — reported as a missing file |
| `test`, `[` | POSIX expressions | an operand, per the POSIX one-argument rule |
| `touch` | `-c` | refused by name |
| `true`, `false` | none, by definition | ignored, which POSIX requires |
| `uname` | `-a -i -m -n -o -p -r -s -v` | refused by name |
| `uniq` | none | refused by name |
| `wc` | `-c -l -w` | refused by name |
| `winpath` | none | treated as a path operand |
| `xargs` | none | refused by name |
| `yes` | none | treated as the string to repeat |

### Options a script is most likely to reach for and not find

`cat -n`, `cp -r`, `mv -f`, `head -c`, `uniq -c`, `basename -a`, `xargs -0`,
`xargs -n`, `sort -k`, `grep -r`, `ls -l` beyond the basic long form, and every
`find` expression. Filling these in is v1.1; see `docs/design/v1-scope.md` and
the per-applet tables in `docs/testing/applet-test-inventory.md`.

### `find` does not honour its expressions

Measured, and worse than a missing option:

```console
$ find . -name m.txt
.
f.txt
m.txt
find: -name: No such file or directory
$ echo $?
1
```

`find` walks and prints the **whole tree**, then treats `-name` as a path
operand and fails. The exit status is non-zero, but the wrong output has already
been written. A pipeline such as `find . -name '*.tmp' | xargs rm` would
therefore receive every file rather than the matching ones.

Until `find` implements expressions or refuses them before walking, treat it as
**equivalent to `find PATH` with no predicates** and do not pipe it into a
command with effects.
