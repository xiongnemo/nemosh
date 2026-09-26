package applets_test

import (
	"os"
	"testing"
)

// Run on its own, with no shell behind it, `test -v` has only the environment to look in.
// Inside the shell it asks the shell; that side is internal/shell/runtime/test_v_test.go.
func TestTest_vLooksInTheEnvironmentWithoutAShell(t *testing.T) {
	t.Setenv("NEMOSH_TEST_V_SET", "")
	os.Unsetenv("NEMOSH_TEST_V_UNSET")
	if status, message := runTestApplet(t, "-v", "NEMOSH_TEST_V_SET"); status != 0 {
		t.Errorf("test -v on a variable set empty: status %d (%q), want 0", status, message)
	}
	if status, message := runTestApplet(t, "-v", "NEMOSH_TEST_V_UNSET"); status != 1 {
		t.Errorf("test -v on an unset variable: status %d (%q), want 1", status, message)
	}
}
