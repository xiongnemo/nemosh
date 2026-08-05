# Initial Behavior Cases

This document lists the first behavior cases to create before or alongside the
first Go runtime code. Each case should become one TOML file under
`tests/behavior/`.

## POSIX Shell Gates

These are intended to follow POSIX shell semantics. They should be compared with
dash and BusyBox ash where possible.

| ID | Assertion | Notes |
| --- | --- | --- |
| `shell.posix.simple-command.status` | A simple command returns its exit status. | `true`, `false`, `exit 7`. |
| `shell.posix.assignment.visibility` | Leading assignments affect the command environment; standalone assignments persist. | Covers `VAR=x cmd` vs `VAR=x`. |
| `shell.posix.quoting.single` | Single quotes preserve literal text. | No expansion inside single quotes. |
| `shell.posix.quoting.double` | Double quotes allow parameter and command substitution but preserve spaces. | Basic word splitting gate. |
| `shell.posix.backslash.escape` | Unquoted backslash escapes the following character. | Also protects Windows decision that `C:\foo` is not raw path syntax. |
| `shell.posix.parameter.default` | `${v:-word}` and `${v-word}` follow unset/null rules. | Start with default-value operators. |
| `shell.posix.positional` | `set --` sets `$#`, `$1`, `$2`, and quoted positional args. | Needed by scripts and tests. |
| `shell.posix.command-substitution.basic` | `$(...)` captures stdout and strips trailing newlines per POSIX rules. | Include quoted and unquoted use. |
| `shell.posix.redirection.output` | `>` creates/truncates and redirects stdout. | File effect expected. |
| `shell.posix.redirection.input` | `<` feeds stdin to a command. | Use `cat` or builtin test helper. |
| `shell.posix.redirection.dup` | `2>&1` duplicates stderr to stdout. | FD table implemented; add the corpus case. |
| `shell.posix.heredoc.quoted` | Quoted here-doc delimiters suppress expansion. | Already covered by smoke probe. |
| `shell.posix.pipeline.status` | Pipeline status is the last command by default. | `false | true` returns 0. |
| `shell.posix.pipeline.pipefail` | `set -o pipefail` changes status as Nemosh extension. | Tag as `nemosh`, not strict POSIX. |
| `shell.posix.and-or` | `&&` and `||` short-circuit. | From existing smoke. |
| `shell.posix.subshell.isolation` | `( x=inner )` does not mutate parent `x`. | Uses state snapshot. |
| `shell.posix.loop.while-read` | `while IFS= read -r line` handles pipeline input. | Tests read, pipeline, loop. |
| `shell.posix.special-builtins.export` | `export` marks variables for child environment. | Runtime and native-child probes exist; add the corpus case. |
| `shell.posix.special-builtins.readonly` | readonly variables reject reassignment. | Include error status. |
| `shell.posix.trap.exit` | `trap '...' EXIT` runs on shell exit. | Windows-supported v0 trap. |

## POSIX Utility Gates

These applet behaviors should follow POSIX where POSIX defines the utility.

| ID | Assertion | Applets |
| --- | --- | --- |
| `applet.posix.test.string` | `[ x = x ]`, `[ x != y ]`, `[ -n x ]`, `[ -z '' ]`. | `[`, `test` |
| `applet.posix.test.file` | `-e`, `-f`, `-d` reflect created fixtures. | `[`, `test` |
| `applet.posix.printf.basic` | `%s`, `%d`, escapes, and no implicit newline. | `printf` |
| `applet.posix.echo.basic` | Basic echo prints arguments and newline. | `echo`; avoid ambiguous `-e` initially. |
| `applet.posix.pwd.logical` | `pwd` prints current logical cwd. | `pwd` |
| `applet.posix.env.print` | `env` prints exported env. | `env`, `printenv` |
| `applet.posix.cat.stdin` | `cat` copies stdin to stdout. | `cat` |
| `applet.posix.wc.lines` | `wc -l` counts lines. | `wc` |
| `applet.posix.basename.dirname` | basename/dirname handle simple paths. | `basename`, `dirname` |
| `applet.posix.mkdir.rm` | create and remove directories/files. | `mkdir`, `rmdir`, `rm` |
| `applet.posix.cp.mv` | copy and rename simple files. | `cp`, `mv` |

## BusyBox-W32 Windows Gates

These intentionally follow busybox-w32 native Windows behavior.

| ID | Assertion | Notes |
| --- | --- | --- |
| `path.windows.current-root.drive` | After `cd D:/dir`, `cd /` returns to `D:/`. | Requires available drive fixture or synthetic temp drive case. |
| `path.windows.current-root.unc` | After `cd //host/share/dir`, `cd /` returns to `//host/share`. | Requires configured network fixture. |
| `path.windows.unc-host-only` | `cd //host` fails with targeted hint. | No network enumeration by default. |
| `path.windows.drive-short` | `/c/foo` resolves to drive C path and displays `/c/foo`. | Nemosh display differs from busybox-w32 `C:/foo`. |
| `path.windows.mnt-alias` | `/mnt/c/foo` resolves as alias for `/c/foo`. | Nemosh configurable default-on alias. |
| `path.windows.cygdrive-off` | `/cygdrive/c/foo` is rejected or disabled by default. | Configurable default-off. |
| `path.windows.virtual-root-priority` | `/dev/null` and `/tmp` do not become current-root-relative paths. | Virtual roots beat current root. |
| `exec.windows.suffix-order` | Lookup tries `.com`, `.exe`, `.sh`, `.bat`, `.cmd` in fixed order. | Do not use arbitrary `PATHEXT` by default. |
| `exec.windows.no-argv-conversion` | External native argv receives `/c/foo` unchanged unless user calls `winpath`. | Must not behave like MSYS2. |
| `env.windows.path-separator` | `PATH` uses semicolon as separator on Windows. | Drive colon conflict avoidance. |
| `env.windows.case-dedupe` | Exported `Path`/`PATH` collisions are deduped at spawn boundary. | Shell variables remain case-sensitive. |
| `script.windows.crlf` | CRLF shell script runs as LF. | Strip `\r` only in CRLF pairs. |

## Nemosh Extension Gates

These are explicit Nemosh extensions or policy choices.

| ID | Assertion | Notes |
| --- | --- | --- |
| `dev.nemosh.random` | `/dev/random` and `/dev/urandom` produce bytes and do not block. | CSPRNG source. |
| `dev.nemosh.zero` | `/dev/zero` produces NUL bytes. | Applet/read tests. |
| `dev.nemosh.clipboard.text` | `/dev/clipboard` reads/writes UTF-8 text when available. | Platform-gated, may be skipped in headless CI. |
| `dev.nemosh.fd` | `/dev/fd/N` maps to the shell fd table. | Runtime coverage exists; add the corpus case. |
| `diag.nemosh.posix-first-hint` | Errors show POSIX-style first line and optional hint. | Debug details behind flag. |
| `net.nemosh.shares-applet` | `shares //host` enumerates shares if extension is enabled. | Post-v0 or optional applet. |
