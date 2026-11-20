# Ubuntu's stock bash startup files

Byte for byte as Ubuntu's bash package ships them, so `TestSession_runsUbuntusStockStartupFiles`
runs what an administrator copies into `~/.nemoshrc`. Do not reformat or edit them; fetch them
again to update.

| file here | Ubuntu installs it as | source | ETag when fetched, 2026-11-21 |
|---|---|---|---|
| `ubuntu-skel.bashrc` | `/etc/skel/.bashrc` | <https://git.launchpad.net/ubuntu/+source/bash/plain/debian/skel.bashrc> | `b488fcc4cee656840d7a756298456eaa243b3e46` |
| `ubuntu-etc-bash.bashrc` | `/etc/bash.bashrc` | <https://git.launchpad.net/ubuntu/+source/bash/plain/debian/etc.bash.bashrc> | `3ac6cb4193283f9967c8d82e49a1e3a0eae36330` |
| `ubuntu-skel.profile` | `/etc/skel/.profile` | <https://git.launchpad.net/ubuntu/+source/bash/plain/debian/skel.profile> | `d89ea5a6e83a2956d7461b547fa0d7d68103b9c9` |

The default branch's `debian/changelog` opened with `bash (5.3-3ubuntu1)`; `skel.bashrc` is
the same on the `ubuntu/noble` (24.04), `ubuntu/jammy` (22.04) and `ubuntu/devel` branches.
What each line of `skel.bashrc` does here is `docs/testing/linux-habits.md`'s "Ubuntu's
.bashrc, line by line".
