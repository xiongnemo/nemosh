package runtime_test

import "testing"

// A shell string holds no NUL byte, in busybox-w32 and bash alike: a command substitution's
// output and a line read are taken without theirs, and `$'...'` drops one its escapes make --
// busybox's answer; bash ends the string there. Here each was kept, so `$(printf 'a\0b')` was
// three bytes long. And `$'\0'` was the two characters `\0`, where both references make NUL of
// it, and an octal escape is one to three digits, the first 0 among them: `$'\0101'` is \010
// and a 1 in both, where it was A.
func TestRuntime_nulBytesAreDroppedAsBothReferencesDropThem(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"x=$(printf '.\\0.'); echo ${#x}", "2\n"},
		{"x=`printf 'a\\0b'`; printf %s \"$x\" | od -A n -t x1", " 61 62\n"},
		{"printf '.\\0.\\n' | { read x; echo ${#x}; }", "2\n"},
		{"printf '.\\0.' | { read -n 3 x; echo ${#x}; }", "2\n"},
		{"x=$'a\\0b'; echo ${#x}; x=$'\\0'; echo \"[${#x}]\"", "2\n[0]\n"},
		{"x=$'a\\x00b\\000c'; echo ${#x}", "3\n"},
		{"printf %s $'\\0101' | od -A n -t x1", " 08 31\n"},
		{"printf %s $'\\101\\1012' | od -A n -t x1", " 41 41 32\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox-w32 answers", stdout, status, test.want)
			}
		})
	}
}

// bash's own two, busybox having neither: `${x@E}` ends the string at a NUL its escapes make,
// and mapfile ends each line at its first NUL -- an element read up to a NUL delimiter, and
// kept without -t, loses the delimiter so. Each kept the NUL. The octal escapes of @E are
// `$'...'`'s.
func TestRuntime_transformEAndMapfileEndAtNUL(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{"x='a\\0101b\\0c'; printf %s \"${x@E}\" | od -A n -t x1", " 61 08 31 62\n"},
		{"printf '.\\0.\\n.\\0.\\n' | { mapfile a; echo ${#a[@]}; printf %s \"${a[@]}\" | od -A n -t x1; }", "2\n 2e 2e\n"},
		{"printf 'a\\0b\\0' | { mapfile -d '' a; echo ${#a[@]} \"${a[0]}${a[1]}\"; }", "2 ab\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script + "\n"); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as bash answers", stdout, status, test.want)
			}
		})
	}
}
