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
  `read_from_fd.py`) are Python programs. They are not copied, and whatever runs these
  cases has to provide them.
- `LICENSE.txt`.

`upstream.json` records the commit, and each file's git mode, sha256 and number of cases.
The tests in `internal/testutil/oilsspec` hold the copy to it, so an edit here fails them.

## Refreshing

Nothing here is edited by hand. To move to a newer Oils, update the clone in
`references/shells/oils` and run

    bash scripts/oils-vendor.sh

It replaces the copy whole from the clone's HEAD, rewrites `upstream.json`, and stages the
result with upstream's executable bits. Then change the commit named in
`THIRD-PARTY-NOTICES.md`, which a test checks against `upstream.json`.
