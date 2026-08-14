# Support Matrix

Everything here was **measured against a built binary on 2026-08-07**, not read
off the source or carried from an earlier document. The probe ran each applet
with an option it could not possibly implement (`-%`), then with each plausible
option letter, and recorded what came back. Where this table says an option is
unsupported, that is an observation.

Re-measure this file rather than editing it by hand when applet coverage
changes; see AGENTS.md, Documentation Hygiene.

## Platforms

A binary being published is not the same as the platform being supported, and
the second column is the one that decides. Four of these ship archives; one is
supported.

| Platform | Status | What that means |
| --- | --- | --- |
| `windows/amd64` | **Supported** | The target. Bugs here are bugs. Behavior corpus, differential suite against busybox-w32/ash, and native path, launch, device, and interrupt tests all run here. |
| `linux/amd64` | **Build and test only**, binary published | CI compiles and runs the full suite, which is what keeps the platform splits from rotting. Not a support commitment: three interactive interrupt tests are skipped here, and the Windows-only surfaces (clipboard device, `ComSpec` batch launch, 8.3 fallback, case-preserving `cd`) have no counterpart. |
| `linux/arm64` | **Compile only**, binary published | Cross-compiled and never executed. |
| `darwin/amd64`, `darwin/arm64` | **Compile only**, binaries published | `GOOS=darwin go build ./...` runs in CI. Nothing is executed. |
| `windows/arm64` | **Untested** | Not built, not run, not claimed. |

Go 1.26, `CGO_ENABLED=0`, single binary, no runtime sidecars.

## Shell

Implemented and covered by the behavior corpus (145 cases) and the differential
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

Beyond POSIX, `history`, `which` and `set -o nocaseglob` are implemented, both
following busybox.

### `kill`

A builtin, as busybox's is (`shell/ash.c:12096`), and for the same reason: `%N`
names a job and only the shell has the job table. busybox's `killcmd` does
nothing but translate `%N` into that job's pids and hand them to the ordinary
`kill` (`:4787-4830`).

Here there is nothing to translate into, because a background job is a goroutine
and has no pid. What it has is its own context, so the signal arrives as a
cancellation — and for the case that matters most that is not a weaker
substitute: an external command in a background job is launched with
`exec.CommandContext` under that context, so cancelling it terminates the real
process.

| Form | Behaviour |
| --- | --- |
| `kill %N` | cancels that job. Every signal cancels; a goroutine has no handler, so telling TERM from KILL would be a promise this cannot keep |
| `kill PID` | `TerminateProcess` on Windows, as busybox does (`win32/process.c:909`), a real signal elsewhere |
| `kill -9`, `kill -TERM`, `kill -SIGTERM` | all accepted; a script writes the number and a person writes the name |
| `kill -l` | lists the signals this shell can act on, not the whole POSIX set |
| a pid that has already exited | refused, not reported as killed — the check busybox makes with `GetExitCodeProcess` first |
| pid `0` or negative | refused on Windows: those mean process groups, which Windows has not got in the POSIX sense. Passed through elsewhere |

`kill` does not claim the job, so a later `wait %N` still finds it.

### Known divergences from bash/dash/ash

- **Parse before effects.** A syntax error anywhere in a script means none of it
  runs. bash and dash execute up to the error. `{echo bad;}` produces no output
  here; both references print nothing either but reach the command first.
- `~user` is left as written. `~` and `~/path` work.
- An alias whose value is not a list of words is refused at definition time,
  because substitution happens after parsing.
- `${#@}` is not pinned; POSIX leaves it unspecified and the references disagree.

## Applets

All 48 registered applets ship. **Name presence is not option parity**, and the
column that matters is the third one.

| Applet | Options implemented | Unknown option is |
| --- | --- | --- |
| `basename` | `-a`, and the `basename PATH [SUFFIX]` form | refused by name |
| `cat` | `-n` | refused by name |
| `chmod` | numeric mode | refused by name |
| `clear` | none | refused by name |
| `cp` | `-r`, `-R` | refused by name |
| `cut` | `-b -c -d -f -n -s` | refused by name |
| `date` | `-d -u` | refused by name |
| `dirname` | none needed | refused by name |
| `echo` | `-n -e` | treated as text, which is what `echo` does |
| `env` | `-i`, and `NAME=VALUE command` | refused by name |
| `find` | `-name`, `-type f\|d\|l`, `-print`, implicit AND | refused **before the walk** |
| `grep` | `-i -n -v`, `--color[=WHEN]` accepted and ignored | refused by name |
| `head` | `-n -c`, and the `-N` form | refused by name |
| `id` | `-u -g -G -n`, and their clusters | refused by name |
| `ln` | `-s` | refused by name |
| `ls` | `-a -h -l -1`, `--color[=always\|never\|auto]` | refused by name |
| `mkdir` | `-m -p -v` | refused by name |
| `mktemp` | `-d -q -u`, and an `XXXXXX` template | refused by name |
| `mv` | `-f`, accepted and already in force | refused by name |
| `pgrep` | `-l -x`, a regular expression on the process name | refused by name |
| `pkill` | `-x` and a leading `-SIG`, a regular expression on the process name | refused by name |
| `posixpath` | none | treated as a path operand |
| `printenv` | none | treated as a variable name |
| `printf` | format string | treated as the format, which is correct |
| `pwd` | `-L -P` both accepted | accepted |
| `readlink` | `-n` | refused by name |
| `realpath` | none | treated as a path operand |
| `rm` | `-f -r` | refused by name |
| `rmdir` | `-p -v` | refused by name |
| `sed` | `s///` substitution | refused by name |
| `seq` | `LAST`, `FIRST LAST`, `FIRST INCREMENT LAST` | read as a number, so a bad one is refused |
| `sleep` | duration operand | reported as an invalid duration |
| `sort` | `-n -r` | refused by name |
| `tail` | `-n`, and the `-N` form | refused by name |
| `test`, `[` | POSIX expressions | an operand, per the POSIX one-argument rule |
| `tee` | `-a` | refused by name |
| `touch` | `-c` | refused by name |
| `tr` | `-d -s -c`, ranges and backslash escapes; not classes | refused by name |
| `true`, `false` | none, by definition | ignored, which POSIX requires |
| `uname` | `-a -i -m -n -o -p -r -s -v` | refused by name |
| `uniq` | `-c` | refused by name |
| `wc` | `-c -l -w` | refused by name |
| `whoami` | none | refused by name |
| `winpath` | none | treated as a path operand |
| `xargs` | none | refused by name |
| `yes` | none | treated as the string to repeat |

### Options a script is most likely to reach for and not find

`xargs -0`, `xargs -n`, `sort -k`, `grep -r`, `tail -c`, and `ls -l` beyond the
basic long form. Every one of them is refused by name, so a script asking for it
fails rather than quietly getting something else.

`tail -c` is worth calling out because `head -c` now exists: head counts bytes
and tail does not, and the asymmetry is deliberate rather than overlooked --
claiming both would be the kind of thing a script discovers the hard way.

Filling in the rest is v1.1; see
`docs/design/v1-scope.md` and the per-applet tables in
`docs/testing/applet-test-inventory.md`.

### `find`

`-name`, `-type`, and `-print` are implemented, combining with the implicit AND
POSIX specifies. `-name` matches the basename, not the path, because busybox
uses `fnmatch` without `FNM_PATHNAME` and a basename carries no separator for
`*` to cross. `-type` classifies `f`, `d`, and `l`; busybox also accepts `b`,
`c`, `s`, and `p`, which are refused by name here rather than answered as though
a block device could never match.

Every other predicate — `-mtime`, `-size`, `-perm`, `-exec`, `-prune`, `-regex`,
`-maxdepth`, and the rest — is **refused before the first directory is read**:

```console
$ find . -mtime 1
find: unrecognized: -mtime
$ echo $?
1
```

That ordering is the fix, not a detail. Until 2026-08-07 `find` honoured no
expression at all: it walked the whole tree, printed every path, and only then
reported the predicate as a missing file. `find . -name '*.tmp' | xargs rm`
therefore received every file. Both halves were measured, and twelve forms —
`.`, `./`, `sub`, `sub/`, and each predicate combination — now match busybox-w32
byte for byte.

Output follows POSIX rather than being cleaned: the path operand is written
exactly as given, then a slash, then the rest. `find .` yields `./a.txt`, not
`a.txt`.
