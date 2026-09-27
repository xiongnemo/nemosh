# The Oils spec suite: how far nemosh is from bash

This page is written from `tests/oils/baseline.json` and `tests/oils/calibration.json`
by `NEMOSH_OILS=update`, and a test fails when it is out of date. What the suite is, and
why none of its cases says what nemosh must do, is in `tests/oils/README.md`.

Measured on Windows, with the cases of Oils `15de8fd` (2026-05-30), against

- bash: GNU bash, version 5.3.15(2)-release (x86_64-pc-cygwin)
- busybox: BusyBox v1.38.0-FRP-6075-g169694ebd (2026-05-06 12:57:04 UTC)

**nemosh passes 1770 of the 2551 cases bash passes: 69.4%.**

Of the other 781, it does 98 the way the files record of ash, which may be busybox's
way, and 683 neither way.

| | cases |
|---|---:|
| in the vendored files | 2781 |
| in files Oils itself does not run | 46 |
| left out on Windows | 67 |
| measured | 2668 |
| that bash passes | 2551 |
| that nemosh passes of those | 1770 |

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
| arith.test.sh | 74 | 74 | 58 | 78.4% | 1 | 44 |
| array-assign.test.sh | 9 | 9 | 2 | 22.2% | 0 | 9 |
| array-assoc.test.sh | 42 | 36 | 31 | 86.1% | 0 | 0 |
| array-basic.test.sh | 5 | 5 | 5 | 100.0% | 0 | 0 |
| array-compat.test.sh | 12 | 12 | 9 | 75.0% | 0 | 1 |
| array-literal.test.sh | 19 | 16 | 8 | 50.0% | 0 | 0 |
| array-sparse.test.sh | 40 | 40 | 22 | 55.0% | 0 | 0 |
| array.test.sh | 78 | 78 | 67 | 85.9% | 1 | 7 |
| assign-deferred.test.sh | 9 | 9 | 6 | 66.7% | 0 | 1 |
| assign-dialects.test.sh | 4 | 4 | 4 | 100.0% | 0 | 0 |
| assign-extended.test.sh | 39 | 34 | 22 | 64.7% | 0 | 0 |
| assign.test.sh | 48 | 45 | 37 | 82.2% | 1 | 28 |
| background.test.sh | 27 | 25 | 24 | 96.0% | 1 | 18 |
| ble-features.test.sh | 9 | 9 | 3 | 33.3% | 3 | 9 |
| ble-idioms.test.sh | 26 | 26 | 16 | 61.5% | 3 | 23 |
| ble-unset.test.sh | 5 | 5 | 0 | 0.0% | 2 | 5 |
| blog1.test.sh | 9 | 9 | 4 | 44.4% | 1 | 8 |
| blog2.test.sh | 8 | 8 | 3 | 37.5% | 0 | 2 |
| bool-parse.test.sh | 8 | 8 | 6 | 75.0% | 0 | 8 |
| brace-expansion.test.sh | 55 | 54 | 47 | 87.0% | 3 | 8 |
| bugs.test.sh | 28 | 28 | 19 | 67.9% | 7 | 28 |
| builtin-bash.test.sh | 13 | 13 | 10 | 76.9% | 0 | 10 |
| builtin-bind.test.sh | 9 | 8 | 0 | 0.0% | 0 | 0 |
| builtin-bracket.test.sh | 47 | 45 | 38 | 84.4% | 0 | 36 |
| builtin-cd.test.sh | 25 | 22 | 11 | 50.0% | 2 | 11 |
| builtin-completion.test.sh | 51 | 50 | 3 | 6.0% | 0 | 0 |
| builtin-dirs.test.sh | 18 | 18 | 5 | 27.8% | 0 | 2 |
| builtin-echo.test.sh | 27 | 27 | 22 | 81.5% | 5 | 27 |
| builtin-eval-source.test.sh | 23 | 21 | 12 | 57.1% | 0 | 13 |
| builtin-fc.test.sh | 14 | 14 | 1 | 7.1% | 1 | 2 |
| builtin-getopts.test.sh | 31 | 31 | 23 | 74.2% | 4 | 31 |
| builtin-history.test.sh | 17 | 15 | 3 | 20.0% | 0 | 1 |
| builtin-kill.test.sh | 20 | 17 | 15 | 88.2% | 0 | 11 |
| builtin-meta-assign.test.sh | 11 | 11 | 11 | 100.0% | 0 | 11 |
| builtin-meta.test.sh | 15 | 15 | 6 | 40.0% | 1 | 13 |
| builtin-misc.test.sh | 7 | 5 | 2 | 40.0% | 0 | 4 |
| builtin-printf.test.sh | 63 | 55 | 46 | 83.6% | 9 | 57 |
| builtin-process.test.sh | 10 | 9 | 5 | 55.6% | 0 | 6 |
| builtin-read.test.sh | 64 | 63 | 48 | 76.2% | 4 | 60 |
| builtin-set.test.sh | 24 | 24 | 17 | 70.8% | 0 | 20 |
| builtin-special.test.sh | 12 | 11 | 6 | 54.5% | 2 | 10 |
| builtin-times.test.sh | 1 | 1 | 0 | 0.0% | 0 | 1 |
| builtin-trap-bash.test.sh | 23 | 23 | 4 | 17.4% | 0 | 4 |
| builtin-trap-err.test.sh | 22 | 22 | 13 | 59.1% | 3 | 22 |
| builtin-trap.test.sh | 33 | 33 | 15 | 45.5% | 4 | 25 |
| builtin-type-bash.test.sh | 21 | 21 | 13 | 61.9% | 1 | 0 |
| builtin-type.test.sh | 4 | 4 | 2 | 50.0% | 0 | 4 |
| builtin-umask.test.sh | 24 | 15 | 9 | 60.0% | 0 | 2 |
| builtin-vars.test.sh | 41 | 39 | 29 | 74.4% | 1 | 23 |
| case_.test.sh | 13 | 12 | 12 | 100.0% | 0 | 9 |
| command-parsing.test.sh | 5 | 5 | 3 | 60.0% | 0 | 5 |
| command-sub.test.sh | 30 | 28 | 24 | 85.7% | 1 | 28 |
| command_.test.sh | 8 | 6 | 3 | 50.0% | 0 | 4 |
| comments.test.sh | 2 | 2 | 2 | 100.0% | 0 | 2 |
| dbracket.test.sh | 49 | 49 | 29 | 59.2% | 0 | 19 |
| divergence.test.sh | 3 | 3 | 1 | 33.3% | 0 | 3 |
| dparen.test.sh | 15 | 14 | 13 | 92.9% | 0 | 0 |
| empty-bodies.test.sh | 3 | 3 | 1 | 33.3% | 2 | 1 |
| errexit-osh.test.sh | 35 | 35 | 31 | 88.6% | 0 | 35 |
| errexit.test.sh | 35 | 34 | 29 | 85.3% | 0 | 35 |
| exit-status.test.sh | 11 | 11 | 10 | 90.9% | 0 | 7 |
| explore-parsing.test.sh | 5 | 5 | 4 | 80.0% | 0 | 4 |
| extglob-files.test.sh | 23 | 23 | 10 | 43.5% | 0 | 0 |
| extglob-match.test.sh | 29 | 29 | 23 | 79.3% | 0 | 0 |
| fatal-errors.test.sh | 5 | 5 | 0 | 0.0% | 3 | 3 |
| for-expr.test.sh | 9 | 8 | 6 | 75.0% | 0 | 0 |
| func-parsing.test.sh | 15 | 15 | 12 | 80.0% | 2 | 13 |
| glob-bash.test.sh | 8 | 8 | 7 | 87.5% | 0 | 8 |
| glob.test.sh | 39 | 37 | 33 | 89.2% | 0 | 36 |
| globignore.test.sh | 18 | 17 | 3 | 17.6% | 1 | 1 |
| globstar.test.sh | 4 | 4 | 3 | 75.0% | 0 | 0 |
| here-doc.test.sh | 32 | 32 | 28 | 87.5% | 1 | 32 |
| if_.test.sh | 5 | 5 | 5 | 100.0% | 0 | 5 |
| interactive.test.sh | 18 | 17 | 2 | 11.8% | 0 | 0 |
| introspect.test.sh | 13 | 12 | 10 | 83.3% | 0 | 2 |
| known-differences.test.sh | 2 | 2 | 1 | 50.0% | 1 | 2 |
| let.test.sh | 2 | 2 | 2 | 100.0% | 0 | 1 |
| loop.test.sh | 29 | 28 | 22 | 78.6% | 1 | 23 |
| nameref.test.sh | 32 | 32 | 28 | 87.5% | 0 | 2 |
| nix-idioms.test.sh | 6 | 6 | 3 | 50.0% | 0 | 0 |
| nocasematch-match.test.sh | 6 | 6 | 6 | 100.0% | 0 | 3 |
| nul-bytes.test.sh | 16 | 16 | 0 | 0.0% | 0 | 12 |
| paren-ambiguity.test.sh | 8 | 8 | 4 | 50.0% | 2 | 8 |
| parse-errors.test.sh | 27 | 25 | 19 | 76.0% | 2 | 22 |
| pipeline.test.sh | 26 | 26 | 23 | 88.5% | 0 | 17 |
| posix.test.sh | 15 | 15 | 13 | 86.7% | 0 | 14 |
| print-source-code.test.sh | 4 | 4 | 3 | 75.0% | 0 | 0 |
| process-sub.test.sh | 9 | 9 | 9 | 100.0% | 0 | 5 |
| prompt.test.sh | 33 | 26 | 0 | 0.0% | 0 | 0 |
| quote.test.sh | 35 | 35 | 33 | 94.3% | 1 | 35 |
| redirect-command.test.sh | 23 | 23 | 22 | 95.7% | 0 | 22 |
| redirect-multi.test.sh | 13 | 13 | 9 | 69.2% | 0 | 7 |
| redirect.test.sh | 39 | 37 | 33 | 89.2% | 1 | 28 |
| regex.test.sh | 37 | 36 | 26 | 72.2% | 0 | 10 |
| serialize.test.sh | 10 | 9 | 7 | 77.8% | 1 | 10 |
| sh-func.test.sh | 12 | 12 | 11 | 91.7% | 0 | 10 |
| sh-options-bash.test.sh | 9 | 9 | 7 | 77.8% | 0 | 0 |
| sh-options.test.sh | 39 | 39 | 22 | 56.4% | 0 | 12 |
| sh-usage.test.sh | 23 | 22 | 18 | 81.8% | 0 | 18 |
| smoke.test.sh | 18 | 18 | 16 | 88.9% | 0 | 16 |
| strict-options.test.sh | 17 | 15 | 13 | 86.7% | 0 | 7 |
| subshell.test.sh | 2 | 2 | 2 | 100.0% | 0 | 2 |
| temp-binding.test.sh | 4 | 3 | 2 | 66.7% | 1 | 4 |
| tilde.test.sh | 14 | 12 | 10 | 83.3% | 1 | 10 |
| toysh-posix.test.sh | 22 | 22 | 12 | 54.5% | 6 | 22 |
| toysh.test.sh | 8 | 7 | 4 | 57.1% | 0 | 1 |
| type-compat.test.sh | 7 | 5 | 4 | 80.0% | 0 | 0 |
| unicode.test.sh | 7 | 2 | 0 | 0.0% | 0 | 2 |
| var-num.test.sh | 5 | 5 | 5 | 100.0% | 0 | 5 |
| var-op-bash.test.sh | 27 | 26 | 19 | 73.1% | 0 | 2 |
| var-op-len.test.sh | 9 | 5 | 3 | 60.0% | 1 | 3 |
| var-op-patsub.test.sh | 28 | 28 | 23 | 82.1% | 1 | 21 |
| var-op-slice.test.sh | 22 | 22 | 19 | 86.4% | 0 | 6 |
| var-op-strip.test.sh | 29 | 29 | 25 | 86.2% | 0 | 27 |
| var-op-test.test.sh | 37 | 37 | 30 | 81.1% | 1 | 20 |
| var-ref.test.sh | 31 | 31 | 19 | 61.3% | 0 | 0 |
| var-sub-quote.test.sh | 41 | 41 | 38 | 92.7% | 0 | 37 |
| var-sub.test.sh | 6 | 6 | 4 | 66.7% | 1 | 4 |
| vars-bash.test.sh | 1 | 1 | 0 | 0.0% | 0 | 0 |
| vars-special.test.sh | 41 | 39 | 21 | 53.8% | 0 | 19 |
| whitespace.test.sh | 5 | 0 | 0 | - | 0 | 0 |
| word-eval.test.sh | 8 | 7 | 7 | 100.0% | 0 | 6 |
| word-split.test.sh | 55 | 53 | 46 | 86.8% | 5 | 53 |
| xtrace.test.sh | 19 | 17 | 9 | 52.9% | 0 | 9 |
| zsh-idioms.test.sh | 3 | 3 | 2 | 66.7% | 0 | 2 |
| all | 2668 | 2551 | 1770 | 69.4% | 98 | 1447 |
