package runtime_test

import (
	"strings"
	"testing"
)

// `declare -f` prints a definition back in bash's shape. It was not an option.
func TestDeclareF_printsTheDefinition(t *testing.T) {
	stdout, status := runScriptCapturing("f() { echo \"hi $1\" 'lit $x' a\\ b; local x=1 >/dev/null 2>&1; }\ndeclare -f f\n")
	want := "f () \n{ \n    echo \"hi $1\" 'lit $x' a\\ b\n    local x=1 >/dev/null 2>&1\n}\n"
	if status != 0 || stdout != want {
		t.Fatalf("declare -f = %d %q, want %q", status, stdout, want)
	}
	if _, status := runScriptCapturing("declare -f nosuch\n"); status != 1 {
		t.Fatalf("declare -f of a name that is not a function = %d, want 1", status)
	}
	if stdout, _ := runScriptCapturing("b() { :; }\na() { :; }\ndeclare -f | grep '()'\n"); stdout != "a () \nb () \n" {
		t.Fatalf("declare -f with no names = %q, want every function in name order", stdout)
	}
}

// What declare -f prints reads back as the same function. Every construct the printer has
// a case for is here, and the copy is run beside the original.
func TestDeclareF_readsBackAsTheSameFunction(t *testing.T) {
	definition := `g() {
  if [ "$1" = a ]; then
    echo A
  elif [ "$1" = b ]; then echo B
  else
    for i in 1 2; do echo "$i"; done
  fi
  case $1 in
    x|y) echo xy ;;
    *) echo "other $1" ;;
  esac
  while read -r l; do echo "[$l]"; done <<END
body $1
END
  cat <<'RAW'
raw $1
RAW
  { echo grp; echo two; } | tr a-z A-Z
  (echo sub) || true
  n=0; until [ $n -ge 2 ]; do n=$((n+1)); done; echo "n=$n"
  for ((j=0; j<2; j++)); do printf '%s,' "$j"; done; echo
  echo "$(echo cs)" ${v:-def} "a\"b" 2>&1
  echo bg & wait
}
`
	script := definition + "copy=$(declare -f g | sed 's/^g ()/h ()/')\neval \"$copy\"\nfor arg in a b x z; do g $arg; done > one\nfor arg in a b x z; do h $arg; done > two\ncmp -s one two && echo same\n"
	dir := t.TempDir()
	stdout, status := runScriptCapturing("cd '" + strings.ReplaceAll(dir, `\`, "/") + "'\n" + script)
	if status != 0 || stdout != "same\n" {
		t.Fatalf("status %d stdout %q, want the printed copy to behave as the original", status, stdout)
	}
}
