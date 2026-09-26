package runtime_test

import "testing"

// After `/` and `//` a replacement pattern's first character is never the separator, so
// `${x////c}` replaces each slash and `${x///}` deletes them; and a quoted slash in a pattern
// with no replacement is a slash. The first was read as an empty pattern that changed nothing,
// and the second as the separator. busybox-w32 and bash agree on every answer here but the
// anchored `/#` and `/%`, which are bash's.
func TestRuntime_replacePatternMayBeASlash(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`x='/_/'; echo ${x////c}; echo ${x//'/'/c}`, "c_c\nc_c\n"},
		{`HOST_PATH=/foo/bar/baz; echo ${HOST_PATH////\\/}; echo ${HOST_PATH//'/'/\\/}`, "\\/foo\\/bar\\/baz\n\\/foo\\/bar\\/baz\n"},
		{`x=a/bc; echo "${x///}" "${x///y}" "${x//'/'}" "${x//a\/b}"`, "abc a/bc abc c\n"},
		// Unchanged: an escaped or quoted slash, and the anchored forms.
		{`x=a/b; echo "${x//a\/b/c}" "${x//\//}" "${x//"/"/Z}" "${x/#a/Z}" "${x/%b/Z}"`, "c ab aZb Z/b a/Z\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
