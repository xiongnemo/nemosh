package runtime_test

import "testing"

// An anchored empty pattern prefixes or appends, as bash has it (busybox-w32 has neither
// anchor): `${x/#/p-}` and `${x/%/-s}`, and on every element of an array or of "$@". Both
// returned the value unchanged, so a prefix meant for every element went on none of them.
func TestRuntime_anAnchoredEmptyPatternPrefixesOrAppends(t *testing.T) {
	for index, test := range []struct{ script, want string }{
		{`x=aa; echo "${x/#/p-}" "${x/%/-s}"`, "p-aa aa-s\n"},
		{`a=(aa bb ''); printf '[%s]' ${a[@]/#/prefix-}; echo`, "[prefix-aa][prefix-bb][prefix-]\n"},
		{`a=(aa bb ''); printf '[%s]' "${a[@]/%/-suffix}"; echo`, "[aa-suffix][bb-suffix][-suffix]\n"},
		{`set -- aa bb; printf '[%s]' "${@/#/--}"; echo`, "[--aa][--bb]\n"},
		{`x=aa; echo "[${x/#/}]" "[${x/%}]" "[${x//}]" "[${x/}]"`, "[aa] [aa] [aa] [aa]\n"},
		{`x=; echo "[${x/#/p}]"`, "[p]\n"},
	} {
		if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
			t.Errorf("%d: %s: got %q/%d, want %q/0, as bash answers", index, test.script, stdout, status, test.want)
		}
	}
}
