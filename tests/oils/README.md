# The Oils spec suite, vendored

These are the shell-language test cases of [Oils](https://github.com/oils-for-unix/oils),
copied unmodified from commit `15de8fd` (2026-05-30). They are Apache-2.0, as nemosh is:
`LICENSE.txt` is upstream's licence, and the notice is in `THIRD-PARTY-NOTICES.md`.

## What they are for

They are a measuring instrument, not part of the corpus. Each case runs as upstream wrote
it, and what comes out is a count: of the cases bash 5.3 passes, how many nemosh passes,
file by file. A case here pins nothing, and nothing in this directory says what nemosh must
do. A behaviour worth keeping is still written into
`internal/shell/runtime/testdata/bash_corpus.json` or the TOML corpus under
`tests/behavior/`, and measured against the references like any other case.
`docs/design/reference-methodology.md` says why the two are kept apart.

## What was copied

- `spec/*.test.sh`: the 134 files whose `## compare_shells:` line names bash, less `ysh-*`
  and `hay*`, which test Oils' own languages. 2781 cases.
- `spec/testdata/`: all of it, since cases source and run the files there by path.
- `spec/bin/builtins-exec-here-doc-helper.sh`: the one helper a case runs by path. The
  helpers cases call by name (`argv.py`, `printenv.py`, `stdout_stderr.py`,
  `read_from_fd.py`) are Python programs. They are not copied: the harness builds
  `internal/testutil/oilsspec/spechelper` and installs it under each of those names.
- `LICENSE.txt`.

`upstream.json` records the commit, and each file's git mode, sha256 and number of cases.
The tests in `internal/testutil/oilsspec` hold the copy to it, so an edit here fails them.
It also lists the names at the top of the Oils tree and in its `spec/`, because a case that
cds to `$REPO_ROOT` lists and globs them there. The harness builds a tree of those names,
empty, with `spec/testdata` and `spec/bin` in it, for `$REPO_ROOT` to name.

## Running

    NEMOSH_OILS=report go test ./internal/testutil/oilsspec/ -run TestOilsSpec -count=1 -timeout 30m

runs every case with nemosh, installed as `bash` because the cases branch on `$SH`, and
reports how many do what the files record of bash, and of ash. The headline is how many
of the cases bash itself passes nemosh passes too, since Oils recorded its expectations
with an older bash, on Linux, and a case bash fails here says nothing about nemosh.

`calibration.json` is what the references did, run through this harness: every case bash
5.3 did not do as the files record of bash, and every case busybox-w32 did not do as they
record of ash. `NEMOSH_OILS=calibrate` writes it, running bash from `NEMOSH_OILS_BASH` and
busybox from `NEMOSH_OILS_BUSYBOX`. It is also how the harness is shown to be faithful:
bash run through it should do what the files record of bash, and on Windows it does in 2552
of the 2669 cases measured. `NEMOSH_OILS_OUT` names a file for every case's result, as JSON.

A case runs as Oils' `test/sh_spec.py` runs it: its code on the shell's stdin, in a
directory of its own that `TMP` names too, with an environment made for it. There is one
difference: `HOME` is the case's directory, where Oils leaves it unset. On Windows every
shell measured here takes the user's profile for an unset `HOME`, and a case that wrote
under `~` would write into the user's real home.

`exclusions.json` leaves out the cases this harness cannot measure on a platform,
whichever shell runs them: on Windows those that use `chmod`, `ln -s`, `ulimit` or
`read_from_fd.py`; off Linux those that read `/proc`; and everywhere the one case about
an unset `HOME`. Each rule says why. A case nemosh fails on purpose, because it refuses
what the case asks, is not excluded: it counts.

## Refreshing

Nothing here is edited by hand. To move to a newer Oils, update the clone in
`references/shells/oils` and run

    bash scripts/oils-vendor.sh

It replaces the copy whole from the clone's HEAD, rewrites `upstream.json`, and stages the
result with upstream's executable bits. Then change the commit named in
`THIRD-PARTY-NOTICES.md`, which a test checks against `upstream.json`.
