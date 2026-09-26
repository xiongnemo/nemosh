package runtime_test

import "testing"

// In a bracket expression an escaped or quoted character is an ordinary member: `[\]]` holds
// `]`, and `[a\-z]` and `[a"-"z]` hold a, - and z rather than the range. The escape was not
// known, so the first bracket closed at the escaped `]`, and a quoted `-` made a range.
// busybox-w32 and bash agree on every answer here.
func TestRuntime_bracketExpressionHonoursEscapes(t *testing.T) {
	tests := []struct {
		script, want string
	}{
		{`case "]" in [\]]) echo y;; *) echo n;; esac`, "y\n"},
		{`case - in [a\-z]) echo y;; *) echo n;; esac; case b in [a\-z]) echo y;; *) echo n;; esac`, "y\nn\n"},
		{`case b in [a"-"z]) echo y;; *) echo n;; esac; case - in [a'-'z]) echo y;; *) echo n;; esac`, "n\ny\n"},
		{`p='^++--hello.,world<>[]'; echo "${p//[^'><+-.,[]']}"`, "++--.,<>[]\n"},
		{`x=a]b; echo "${x//[\]]/X}" "${x//[a\]]/Y}"`, "aXb YYb\n"},
		// Unchanged: a range, a class, and a negation.
		{`case m in [a-z]) echo y;; esac; case 5 in [[:digit:]]) echo y;; esac; case x in [!a]) echo y;; esac`, "y\ny\ny\n"},
	}
	for _, test := range tests {
		t.Run(test.script, func(t *testing.T) {
			if stdout, status := runScriptCapturing(test.script); stdout != test.want || status != 0 {
				t.Errorf("got %q/%d, want %q/0, as busybox and bash answer", stdout, status, test.want)
			}
		})
	}
}
