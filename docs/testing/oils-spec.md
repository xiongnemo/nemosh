# The Oils spec suite: how far nemosh is from bash

This page is written from `tests/oils/baseline.json` and `tests/oils/calibration.json`
by `NEMOSH_OILS=update`, and a test fails when it is out of date. What the suite is, and
why none of its cases says what nemosh must do, is in `tests/oils/README.md`.

Measured on Windows, with the cases of Oils `15de8fd` (2026-05-30), against

- bash: GNU bash, version 5.3.15(2)-release (x86_64-pc-cygwin)
- busybox: BusyBox v1.38.0-FRP-6075-g169694ebd (2026-05-06 12:57:04 UTC)

**nemosh passes 1560 of the 2551 cases bash passes: 61.2%.**

Of the other 991, it does 101 the way the files record of ash, which may be busybox's
way, and 890 neither way.

| | cases |
|---|---:|
| in the vendored files | 2781 |
| in files Oils itself does not run | 46 |
| left out on Windows | 67 |
| measured | 2668 |
| that bash passes | 2551 |
| that nemosh passes of those | 1560 |

Left out on Windows, as cases no shell can be measured on there, for the reasons
`tests/oils/exclusions.json` gives:

- /proc: 3
- HOME: 1
- descriptors past 2: 4
- executable bits: 34
- resource limits: 17
- symbolic links: 7
- writing at the root: 1

## File by file

Of each file's measured cases: how many bash passes; how many of those nemosh passes,
and does the way ash does instead; and how many busybox does the way the file records
of ash.

| file | measured | bash | nemosh | % | as ash | busybox |
|---|---:|---:|---:|---:|---:|---:|
| alias.test.sh | 48 | 47 | 22 | 46.8% | 0 | 42 |
| append.test.sh | 20 | 20 | 15 | 75.0% | 0 | 1 |
| arg-parse.test.sh | 3 | 3 | 1 | 33.3% | 2 | 3 |
| arith-context.test.sh | 16 | 16 | 11 | 68.8% | 0 | 4 |
| arith-dynamic.test.sh | 4 | 4 | 1 | 25.0% | 0 | 1 |
| arith.test.sh | 74 | 74 | 55 | 74.3% | 1 | 44 |
| array-assign.test.sh | 9 | 9 | 2 | 22.2% | 1 | 9 |
| array-assoc.test.sh | 42 | 36 | 25 | 69.4% | 0 | 0 |
| array-basic.test.sh | 5 | 5 | 5 | 100.0% | 0 | 0 |
| array-compat.test.sh | 12 | 12 | 8 | 66.7% | 0 | 1 |
| array-literal.test.sh | 19 | 16 | 7 | 43.8% | 0 | 0 |
| array-sparse.test.sh | 40 | 40 | 21 | 52.5% | 0 | 0 |
| array.test.sh | 78 | 78 | 61 | 78.2% | 2 | 7 |
| assign-deferred.test.sh | 9 | 9 | 3 | 33.3% | 1 | 1 |
| assign-dialects.test.sh | 4 | 4 | 1 | 25.0% | 0 | 0 |
| assign-extended.test.sh | 39 | 34 | 18 | 52.9% | 0 | 0 |
| assign.test.sh | 48 | 45 | 31 | 68.9% | 1 | 28 |
| background.test.sh | 27 | 25 | 21 | 84.0% | 1 | 18 |
| ble-features.test.sh | 9 | 9 | 3 | 33.3% | 3 | 9 |
| ble-idioms.test.sh | 26 | 26 | 14 | 53.8% | 3 | 23 |
| ble-unset.test.sh | 5 | 5 | 0 | 0.0% | 2 | 5 |
| blog1.test.sh | 9 | 9 | 3 | 33.3% | 1 | 8 |
| blog2.test.sh | 8 | 8 | 3 | 37.5% | 0 | 2 |
| bool-parse.test.sh | 8 | 8 | 6 | 75.0% | 0 | 8 |
| brace-expansion.test.sh | 55 | 54 | 44 | 81.5% | 3 | 8 |
| bugs.test.sh | 28 | 28 | 19 | 67.9% | 7 | 28 |
| builtin-bash.test.sh | 13 | 13 | 9 | 69.2% | 0 | 10 |
| builtin-bind.test.sh | 9 | 8 | 0 | 0.0% | 0 | 0 |
| builtin-bracket.test.sh | 47 | 45 | 36 | 80.0% | 0 | 36 |
| builtin-cd.test.sh | 25 | 22 | 11 | 50.0% | 2 | 11 |
| builtin-completion.test.sh | 51 | 50 | 3 | 6.0% | 0 | 0 |
| builtin-dirs.test.sh | 18 | 18 | 5 | 27.8% | 0 | 2 |
| builtin-echo.test.sh | 27 | 27 | 16 | 59.3% | 2 | 27 |
| builtin-eval-source.test.sh | 23 | 21 | 10 | 47.6% | 0 | 13 |
| builtin-fc.test.sh | 14 | 14 | 1 | 7.1% | 1 | 2 |
| builtin-getopts.test.sh | 31 | 31 | 22 | 71.0% | 4 | 31 |
| builtin-history.test.sh | 17 | 15 | 2 | 13.3% | 0 | 1 |
| builtin-kill.test.sh | 20 | 17 | 5 | 29.4% | 0 | 11 |
| builtin-meta-assign.test.sh | 11 | 11 | 4 | 36.4% | 1 | 11 |
| builtin-meta.test.sh | 15 | 15 | 4 | 26.7% | 5 | 13 |
| builtin-misc.test.sh | 7 | 5 | 2 | 40.0% | 0 | 4 |
| builtin-printf.test.sh | 63 | 55 | 40 | 72.7% | 7 | 57 |
| builtin-process.test.sh | 10 | 9 | 5 | 55.6% | 0 | 6 |
| builtin-read.test.sh | 64 | 63 | 47 | 74.6% | 4 | 60 |
| builtin-set.test.sh | 24 | 24 | 17 | 70.8% | 0 | 20 |
| builtin-special.test.sh | 12 | 11 | 5 | 45.5% | 2 | 10 |
| builtin-times.test.sh | 1 | 1 | 0 | 0.0% | 0 | 1 |
| builtin-trap-bash.test.sh | 23 | 23 | 4 | 17.4% | 0 | 4 |
| builtin-trap-err.test.sh | 22 | 22 | 13 | 59.1% | 3 | 22 |
| builtin-trap.test.sh | 33 | 33 | 15 | 45.5% | 4 | 25 |
| builtin-type-bash.test.sh | 21 | 21 | 12 | 57.1% | 1 | 0 |
| builtin-type.test.sh | 4 | 4 | 2 | 50.0% | 0 | 4 |
| builtin-umask.test.sh | 24 | 15 | 5 | 33.3% | 1 | 2 |
| builtin-vars.test.sh | 41 | 39 | 26 | 66.7% | 1 | 23 |
| case_.test.sh | 13 | 12 | 11 | 91.7% | 0 | 9 |
| command-parsing.test.sh | 5 | 5 | 3 | 60.0% | 0 | 5 |
| command-sub.test.sh | 30 | 28 | 24 | 85.7% | 1 | 28 |
| command_.test.sh | 8 | 6 | 2 | 33.3% | 0 | 4 |
| comments.test.sh | 2 | 2 | 2 | 100.0% | 0 | 2 |
| dbracket.test.sh | 49 | 49 | 27 | 55.1% | 0 | 19 |
| divergence.test.sh | 3 | 3 | 1 | 33.3% | 0 | 3 |
| dparen.test.sh | 15 | 14 | 13 | 92.9% | 0 | 0 |
| empty-bodies.test.sh | 3 | 3 | 1 | 33.3% | 2 | 1 |
| errexit-osh.test.sh | 35 | 35 | 30 | 85.7% | 0 | 35 |
| errexit.test.sh | 35 | 34 | 29 | 85.3% | 0 | 35 |
| exit-status.test.sh | 11 | 11 | 10 | 90.9% | 0 | 7 |
| explore-parsing.test.sh | 5 | 5 | 4 | 80.0% | 0 | 4 |
| extglob-files.test.sh | 23 | 23 | 8 | 34.8% | 0 | 0 |
| extglob-match.test.sh | 29 | 29 | 23 | 79.3% | 0 | 0 |
| fatal-errors.test.sh | 5 | 5 | 0 | 0.0% | 3 | 3 |
| for-expr.test.sh | 9 | 8 | 6 | 75.0% | 0 | 0 |
| func-parsing.test.sh | 15 | 15 | 12 | 80.0% | 2 | 13 |
| glob-bash.test.sh | 8 | 8 | 4 | 50.0% | 3 | 8 |
| glob.test.sh | 39 | 37 | 29 | 78.4% | 0 | 36 |
| globignore.test.sh | 18 | 17 | 3 | 17.6% | 1 | 1 |
| globstar.test.sh | 4 | 4 | 3 | 75.0% | 0 | 0 |
| here-doc.test.sh | 32 | 32 | 28 | 87.5% | 1 | 32 |
| if_.test.sh | 5 | 5 | 5 | 100.0% | 0 | 5 |
| interactive.test.sh | 18 | 17 | 2 | 11.8% | 0 | 0 |
| introspect.test.sh | 13 | 12 | 10 | 83.3% | 0 | 2 |
| known-differences.test.sh | 2 | 2 | 1 | 50.0% | 1 | 2 |
| let.test.sh | 2 | 2 | 2 | 100.0% | 0 | 1 |
| loop.test.sh | 29 | 28 | 22 | 78.6% | 0 | 23 |
| nameref.test.sh | 32 | 32 | 6 | 18.8% | 0 | 2 |
| nix-idioms.test.sh | 6 | 6 | 0 | 0.0% | 0 | 0 |
| nocasematch-match.test.sh | 6 | 6 | 6 | 100.0% | 0 | 3 |
| nul-bytes.test.sh | 16 | 16 | 0 | 0.0% | 0 | 12 |
| paren-ambiguity.test.sh | 8 | 8 | 4 | 50.0% | 2 | 8 |
| parse-errors.test.sh | 27 | 25 | 16 | 64.0% | 1 | 22 |
| pipeline.test.sh | 26 | 26 | 21 | 80.8% | 0 | 17 |
| posix.test.sh | 15 | 15 | 13 | 86.7% | 0 | 14 |
| print-source-code.test.sh | 4 | 4 | 3 | 75.0% | 0 | 0 |
| process-sub.test.sh | 9 | 9 | 9 | 100.0% | 0 | 5 |
| prompt.test.sh | 33 | 26 | 0 | 0.0% | 0 | 0 |
| quote.test.sh | 35 | 35 | 31 | 88.6% | 1 | 35 |
| redirect-command.test.sh | 23 | 23 | 22 | 95.7% | 0 | 22 |
| redirect-multi.test.sh | 13 | 13 | 8 | 61.5% | 0 | 7 |
| redirect.test.sh | 39 | 37 | 33 | 89.2% | 1 | 28 |
| regex.test.sh | 37 | 36 | 25 | 69.4% | 0 | 10 |
| serialize.test.sh | 10 | 9 | 3 | 33.3% | 1 | 10 |
| sh-func.test.sh | 12 | 12 | 11 | 91.7% | 0 | 10 |
| sh-options-bash.test.sh | 9 | 9 | 1 | 11.1% | 0 | 0 |
| sh-options.test.sh | 39 | 39 | 12 | 30.8% | 1 | 12 |
| sh-usage.test.sh | 23 | 22 | 18 | 81.8% | 0 | 18 |
| smoke.test.sh | 18 | 18 | 16 | 88.9% | 0 | 16 |
| strict-options.test.sh | 17 | 15 | 13 | 86.7% | 0 | 7 |
| subshell.test.sh | 2 | 2 | 2 | 100.0% | 0 | 2 |
| temp-binding.test.sh | 4 | 3 | 2 | 66.7% | 1 | 4 |
| tilde.test.sh | 14 | 12 | 6 | 50.0% | 1 | 10 |
| toysh-posix.test.sh | 22 | 22 | 10 | 45.5% | 5 | 22 |
| toysh.test.sh | 8 | 7 | 2 | 28.6% | 0 | 1 |
| type-compat.test.sh | 7 | 5 | 4 | 80.0% | 0 | 0 |
| unicode.test.sh | 7 | 2 | 0 | 0.0% | 0 | 2 |
| var-num.test.sh | 5 | 5 | 5 | 100.0% | 0 | 5 |
| var-op-bash.test.sh | 27 | 26 | 17 | 65.4% | 0 | 2 |
| var-op-len.test.sh | 9 | 5 | 3 | 60.0% | 1 | 3 |
| var-op-patsub.test.sh | 28 | 28 | 19 | 67.9% | 0 | 21 |
| var-op-slice.test.sh | 22 | 22 | 19 | 86.4% | 0 | 6 |
| var-op-strip.test.sh | 29 | 29 | 25 | 86.2% | 0 | 27 |
| var-op-test.test.sh | 37 | 37 | 26 | 70.3% | 0 | 20 |
| var-ref.test.sh | 31 | 31 | 6 | 19.4% | 0 | 0 |
| var-sub-quote.test.sh | 41 | 41 | 30 | 73.2% | 1 | 37 |
| var-sub.test.sh | 6 | 6 | 4 | 66.7% | 1 | 4 |
| vars-bash.test.sh | 1 | 1 | 0 | 0.0% | 0 | 0 |
| vars-special.test.sh | 41 | 39 | 22 | 56.4% | 0 | 19 |
| whitespace.test.sh | 5 | 0 | 0 | - | 0 | 0 |
| word-eval.test.sh | 8 | 7 | 5 | 71.4% | 0 | 6 |
| word-split.test.sh | 55 | 53 | 35 | 66.0% | 4 | 53 |
| xtrace.test.sh | 19 | 17 | 9 | 52.9% | 0 | 9 |
| zsh-idioms.test.sh | 3 | 3 | 2 | 66.7% | 0 | 2 |
| all | 2668 | 2551 | 1560 | 61.2% | 101 | 1447 |
