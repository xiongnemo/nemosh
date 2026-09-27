package runtime_test

import (
	"strconv"
	"testing"
)

// pushd, popd and dirs read their words as bash's pushd.def does; busybox has none of the
// three. `--` ends the options and `-n` moves no directory. A `+N` or `-N` that is not a
// number is an invalid number -- `-z` and `-lv` included -- any other word popd is given an
// invalid argument, and any dirs does not know an invalid option, each said with the usage
// line and status 2. `pushd -- dir`, `popd --` and `-n` were refused, and every misuse was
// status 1. Each transcript is bash 5.3's.
func TestDirs_readsItsWordsAsBashDoes(t *testing.T) {
	for index, test := range []struct {
		script, want, says string
	}{
		{"pushd -- a >/dev/null; echo st=$?; pwd", "st=0\nR/a\n", ""},
		{"pushd a >/dev/null; popd -- >/dev/null; echo st=$?; pwd", "st=0\nR\n", ""},
		{"pushd -n a; echo st=$?; pwd", "R a\nst=0\nR\n", ""},
		{"pushd -n; echo st=$?", "st=0\n", ""},
		{"pushd a >/dev/null; popd -n >/dev/null; echo st=$?; pwd; dirs", "st=0\nR/a\nR/a\n", ""},
		{"pushd a >/dev/null; pushd ../b >/dev/null; popd +1 +0 >/dev/null; pwd; dirs", "R/a\nR/a R\n", ""},
		{"pushd a >/dev/null; pushd ../b >/dev/null; popd -0; echo st=$?", "R/b R/a\nst=0\n", ""},
		{"pushd a >/dev/null; pushd +1 zzz; echo st=$?", "R R/a\nst=0\n", ""},
		{"pushd a >/dev/null; pushd -n +1; echo st=$?; pwd; dirs", "st=0\nR/a\nR/a R/a\n", ""},
		{"pushd a >/dev/null; pushd +0; echo st=$?", "R/a R\nst=0\n", ""},
		{"pushd -z; echo st=$?", "st=2\n", "pushd: -z: invalid number\npushd: usage: pushd [-n] [+N | -N | dir]\n"},
		{"pushd -- -z; echo st=$?", "st=1\n", "pushd: -z: No such file or directory\n"},
		{"pushd a b; echo st=$?", "st=1\n", "pushd: too many arguments\n"},
		{"pushd a >/dev/null; popd zzz; echo st=$?", "st=2\n", "popd: zzz: invalid argument\npopd: usage: popd [-n] [+N | -N]\n"},
		{"popd -z; echo st=$?", "st=2\n", "popd: -z: invalid number\npopd: usage: popd [-n] [+N | -N]\n"},
		{"popd -- zzz; echo st=$?", "st=1\n", "popd: directory stack empty\n"},
		{"dirs -lv; echo st=$?", "st=2\n", "dirs: -lv: invalid number\ndirs: usage: dirs [-clpv] [+N] [-N]\n"},
		{"dirs zzz; echo st=$?", "st=2\n", "dirs: zzz: invalid option\ndirs: usage: dirs [-clpv] [+N] [-N]\n"},
		{"dirs --; echo st=$?", "R\nst=0\n", ""},
		{"pushd a >/dev/null; dirs -v +1; dirs +9; echo st=$?", " 1  R\nst=1\n", "dirs: 9: directory stack index out of range\n"},
		{"dirs +1; echo st=$?; pushd +1; echo st=$?", "st=1\nst=1\n", "dirs: directory stack empty\npushd: directory stack empty\n"},
		{"pushd a >/dev/null; pushd ../b >/dev/null; dirs +1 -0", "R\n", ""},
	} {
		// Named by number: t.TempDir takes the name, and a `$` in it would be expanded by the
		// prefix's cd.
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			prefix, _, hide := dirsRoot(t)
			status, stdout, stderr := runSetScript(t, prefix+test.script+"\n")
			if got, said := hide(stdout), hide(stderr); status != 0 || got != test.want || said != test.says {
				t.Errorf("%s\n  got %q, said %q, status %d\n  want %q, saying %q", test.script, got, said, status, test.want, test.says)
			}
		})
	}
}
