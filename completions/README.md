# Completion specs

One file per command, named after the command: `adb.toml` describes `adb`. The
file name **is** the lookup key — nothing scans this directory, and nothing is
read until the first Tab for that command.

These describe commands Nemosh does **not** ship. Its own applets and builtins
are in `internal/capability`, where a test runs each one and fails if the table
claims an option it refuses. Nothing here can be checked that way, which is what
the rest of this file is about.

## Where they are looked for

| | |
| --- | --- |
| `%APPDATA%\nemosh\completions\<cmd>.toml` | yours, on Windows |
| `~/.config/nemosh/completions/<cmd>.toml` | yours, elsewhere |
| this directory | bundled, compiled into the binary |

Yours wins. That is the whole fix for a bundled spec that is wrong for your
machine, and it is not hypothetical — see `wget.toml`.

Bundled specs are **embedded**, not installed beside the executable. Nemosh is
one static binary with no runtime sidecars, and a directory that had to travel
with `nemosh.exe` would end that.

## The format

```toml
[meta]
derived-from = "adb --help"        # the command whose output this was read off
tool-version = "Android Debug Bridge version 1.0.41 / Version 37.0.0-14910828"
measured-on  = "2026-08-16"

[command]
name        = "adb"                 # must equal the file name
operand     = "none"                # path | directory | host | none
short       = "adestHPL"            # every accepted option letter
value-short = "stHPL"               # those that consume the following word
file-short  = "..."                 # those whose consumed word is a path
long        = ["one-device"]        # names written without their dashes
value-long  = ["one-device"]
file-long   = []

[[subcommand]]                      # a word that selects a different surface
name    = "install"
operand = "path"
short   = "lrtsdg"
```

`value-*` must be a subset of the accepted options, and `file-*` a subset of
`value-*`. An unknown key is an error, not a shrug: a misspelled key that is
silently dropped leaves a file that looks right, completes nothing, and gives
its author no reason why.

**Under-claiming is safe.** An option left out simply is not offered. Claiming
one the command does not take is the mistake that matters, because the reader
acts on it.

## `[meta]` is required, and it is the whole discipline

Nothing in CI can run `adb` to check a word of these files. The only defence
against them rotting is that each says which version of which program it was
read off, and when.

Two measurements from the day this directory was created make the point:

- **`wget` is not one program.** On the machine this was written on it resolves
  to *busybox's* applet — four long options — not GNU wget, which has some two
  hundred. A spec for either is wrong for the other, and the name does not say
  which is installed.
- **`curl` is not one build.** `/mingw64/bin/curl` is 8.16.0; `curl` resolved
  through the system PATH is Windows' own 8.13.0. Same program, different
  vintage, different option set at the edges.

So the honest fix for both is not a better bundled file. It is:

```console
$ python3 scripts/completions/generate.py curl > ~/.config/nemosh/completions/curl.toml
```

which reads the binary *you* have and writes the provenance to match.

## Generating instead of transcribing

`scripts/completions/generate.py` reads a command's own `--help` and writes a
spec. curl has 260 long options; transcribing those by hand would be wrong
before it was finished.

It runs **here, offline, when a person asks**. The shell never runs a program to
answer a Tab — that is the rule that keeps the suggestion engine from stuttering,
and it is not negotiable for a keystroke path.

The generator handles the two shapes seen so far: an option table of
`-x, --name <arg>  description` lines, and bare `-x  description` lines. It does
not handle a `usage:` synopsis with bracket groups (`ssh`) or a sectioned
subcommand listing (`adb`); those two were read by hand, which is why their
`[meta]` says so.

## What a spec cannot say yet

The interesting completions for `adb` are not options at all: the serial numbers
`-s` wants, the package names `uninstall` wants, the remote paths `pull` wants.
None of that is in any help text, and it can only come from running `adb`.

That is a later phase: a declared external source, as an **argument vector**
rather than a shell string, so a spec cannot smuggle a second command past its
reader, run only on Tab and never on the suggestion path, with a timeout and a
cache. bash-completion's specs are arbitrary shell code by comparison.
