package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// In bash mode the assignments before eval, `.`, source and unset are a scope of their own for
// the builtin's run, as bash's tempenv is, and a function call's are in the call's scope. local
// makes a prefix binding of the running call's its own where it stands and starts a new local
// with an outer call's value; unset takes away the innermost binding of a name, but empties a
// local of the running call's. Expected outputs are Git for Windows' bash 5.3's, measured
// 2026-11-22; the scripts are Oils' ble-unset, ble-features and temp-binding cases.
func TestTempenvScope_bashModeIsBashs(t *testing.T) {
	for _, test := range []struct{ name, code, stdout string }{
		{"eval's local is the eval's", `f1() {
  local v=local1
  v=tempenv2 eval 'echo "a $v"; local v=local2; echo "b $v"'
  echo "c $v"
}
v=global
v=tempenv1 f1`, "a tempenv2\nb local2\nc local1\n"},
		{"unset and unlocal in an eval's local", `unlocal() { unset -v "$1"; }
f2() {
  local v=local1
  v=tempenv2 eval '
    local v=local2
    (unset v; echo "unset ${v-(unset)}")
    (unlocal v; echo "unlocal ${v-(unset)}")
  '
}
v=global
v=tempenv1 f2`, "unset (unset)\nunlocal local1\n"},
		{"unlocal through nested locals", `unlocal() { unset -v "$1"; }
f3() {
  local v=local1
  v=tempenv2 eval '
    local v=local2
    v=tempenv3 eval "
      local v=local3
      echo \${v-(unset)}; unlocal v; echo \${v-(unset)}; unlocal v; echo \${v-(unset)}
      unlocal v; echo \${v-(unset)}; unlocal v; echo \${v-(unset)}
    "
  '
}
v=global
v=tempenv1 f3`, "local3\nlocal2\nlocal1\nglobal\n(unset)\n"},
		{"unset through nested tempenvs", `f4() {
  v=tempenv2 eval '
    v=tempenv3 eval "
      echo \${v-(unset)}; unset v; echo \${v-(unset)}; unset v; echo \${v-(unset)}
      unset v; echo \${v-(unset)}; unset v; echo \${v-(unset)}
    "
  '
}
v=global
v=tempenv1 f4`, "tempenv3\ntempenv2\ntempenv1\nglobal\n(unset)\n"},
		{"a prefix on unset, local and eval", `f() {
  local x=x
  tmp='' local tx=tx
  eval 'local y=y'
  tmp='' eval 'local ty=ty'
  x='X' unset x; tx='TX' unset tx; y='Y' unset y; ty='TY' unset ty
  echo "$x $tx $y $ty"
}
f`, "x tx y ty\n"},
		{"unset's prefix shadows", `shadow() {
  x=42; x=tmp unset x; echo "x=$x"
  x=42; x=tmp eval 'unset x'; echo "x=$x"
}
shadow; set -o posix; shadow`, "x=42\nx=42\nx=\nx=\n"},
		{"a call's prefix and unset", `unlocal() { unset -v "$1"; }
f1() { local v=local; unset v; echo "a ${v-(unset)}"; }
v=global; v=tempenv f1
f1() { local v=local; unlocal v; echo "b ${v-(unset)}"; }
v=global; v=tempenv f1
f1() { unset v; echo "c ${v-(unset)}"; }
v=global; v=tempenv f1
f1() { unlocal v; echo "d ${v-(unset)}"; }
v=global; v=tempenv f1`, "a (unset)\nb global\nc global\nd global\n"},
		{"local over an eval's prefix", `unlocal() { unset -v "$1"; }
f5() {
  echo "a ${v-(unset)}"; local v; echo "b ${v-(unset)}"
  ( unset v; echo "c ${v-(unset)}" ); ( unlocal v; echo "d ${v-(unset)}" )
}
v=global
v=tempenv eval 'f5'`, "a tempenv\nb tempenv\nc (unset)\nd tempenv\n"},
		{"local inherits an outer call's prefix", `f1() { local v; echo "${v-(unset)}"; }
f2() { f1; }
f3() { local v=local; f1; }
v=global
v=tempenv f2; f3`, "tempenv\n(unset)\n"},
	} {
		var stdout, stderr bytes.Buffer
		rt := New(applets.DefaultRegistry, Streams{Stdout: &stdout, Stderr: &stderr})
		rt.EnterBashMode()
		status := rt.RunScript(context.Background(), test.code+"\n")
		if status != 0 || stdout.String() != test.stdout || stderr.Len() != 0 {
			t.Errorf("%s: status %d, stdout %q, stderr %q; want %q", test.name, status, stdout.String(), stderr.String(), test.stdout)
		}
	}
}
