package applets_test

import (
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// expr says busybox's words where it has nothing to evaluate and where a sum has a word in it,
// "too few arguments" and "non-numeric argument", with busybox's status, 2. They were GNU's,
// "missing operand" and "non-integer argument". Each was measured against busybox-w32.
func TestExpr_saysBusyboxsWords(t *testing.T) {
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: nil, want: "too few arguments"},
		{args: []string{"a", "+", "1"}, want: "non-numeric argument"},
		{args: []string{"1", "/", "0"}, want: "division by zero"},
	} {
		_, _, err := runFilter(t, "expr", test.args, "")
		message, _ := applets.StatusMessage(err)
		if status, _ := applets.StatusCode(err); message != test.want || status != 2 {
			t.Errorf("expr %q = %q, status %d; want %q, status 2", test.args, message, status, test.want)
		}
	}
}
