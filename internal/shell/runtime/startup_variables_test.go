package runtime_test

import (
	"os"
	goruntime "runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// A new shell has bash's UID and EUID, integers and read-only, and OSTYPE: the uid `id -u`
// answers, the one the shell acts as, and msys on Windows. None was set, and none is exported.
func TestRuntime_identityVariables(t *testing.T) {
	uid := applets.CurrentUserID()
	euid := os.Geteuid()
	if euid < 0 {
		euid = uid
	}
	ostype := map[string]string{"windows": "msys", "linux": "linux-gnu"}[goruntime.GOOS]
	if ostype == "" {
		ostype = goruntime.GOOS
	}

	// When
	status, stdout, stderr := runSetScript(t,
		"echo \"[$UID] [$EUID] [$OSTYPE]\"\n(UID=1; echo changed) 2>/dev/null || echo held\ndeclare -p UID\nenv | grep -E '^(UID|EUID|OSTYPE)=' || echo none-exported\n")

	// Then
	want := "[" + strconv.Itoa(uid) + "] [" + strconv.Itoa(euid) + "] [" + ostype + "]\nheld\n" +
		"declare -ir UID=\"" + strconv.Itoa(uid) + "\"\nnone-exported\n"
	if stdout != want || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}

// A new shell has OPTIND=1, PS4="+ " and HOSTNAME, as busybox's has them (shell/ash.c's
// init): none of them was set, so `echo $OPTIND` before a getopts loop was empty and `set -x`
// had no PS4 to show. None is exported by being set.
func TestRuntime_startupVariables(t *testing.T) {
	hostname, err := os.Hostname()
	if err != nil {
		t.Skip("no host name to compare with")
	}

	// When
	status, stdout, stderr := runSetScript(t,
		"echo \"[$OPTIND] [$PS4] [$HOSTNAME]\"\nenv | grep -E '^(OPTIND|PS4)=' || echo none-exported\n")

	// Then
	if want := "[1] [+ ] [" + hostname + "]\nnone-exported\n"; !strings.EqualFold(stdout, want) || status != 0 {
		t.Fatalf("got %q/%d, want %q/0; stderr = %q", stdout, status, want, stderr)
	}
}
