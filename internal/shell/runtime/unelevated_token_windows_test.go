//go:build windows

package runtime

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// What drop runs has a token Windows calls unelevated, which an elevated shell's is not. These say
// little where the test runs unelevated, as on a developer's machine, and everything on a CI
// runner, which runs elevated.
func TestUnelevatedToken_isOneWindowsCallsUnelevated(t *testing.T) {
	// When
	token, err := unelevatedToken()
	if err != nil {
		t.Fatalf("unelevatedToken: %v", err)
	}
	defer token.Close()

	// Then
	if token.IsElevated() {
		t.Fatal("Windows calls the token elevated")
	}
	groups, err := token.GetTokenGroups()
	if err != nil {
		t.Fatal(err)
	}
	for _, group := range groups.AllGroups() {
		switch group.Sid.String() {
		case "S-1-5-32-544", "S-1-5-114":
			if group.Attributes&windows.SE_GROUP_ENABLED != 0 {
				t.Errorf("%s is enabled, attributes %#x", group.Sid, group.Attributes)
			}
		}
	}
	if level := tokenIntegrityLevel(t, token); level > mediumIntegrity {
		t.Errorf("integrity level %#x, want medium at most", level)
	}
	user, err := token.GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	if owner := tokenOwner(t, token); !owner.Equals(user.User.Sid) {
		t.Errorf("objects it makes are owned by %s, not by %s, so it could not open the processes it starts", owner, user.User.Sid)
	}
}

// drop runs the command with that token, and the command can start programs of its own, which it
// could not when its objects were owned by the Administrators group it may no longer use.
func TestDrop_runsACommandThatCanStartPrograms(t *testing.T) {
	binary := os.Getenv(jobBinaryVariable)
	if binary == "" {
		t.Skip("no nemosh was built for this run")
	}
	whoami := strings.ReplaceAll(os.Getenv("SystemRoot"), `\`, "/") + "/System32/whoami.exe"

	// When
	output, err := exec.Command(binary, "drop", "-c", `id -u && "$1" >/dev/null && echo started; exit 3`, "drop", whoami).CombinedOutput()

	// Then
	if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 3 {
		t.Fatalf("drop: %v, output %q; want status 3", err, output)
	}
	if string(output) != "4095\nstarted\n" {
		t.Fatalf("output %q, want an unprivileged id and the program it started", output)
	}
}

func tokenIntegrityLevel(t *testing.T, token windows.Token) uint32 {
	t.Helper()
	buffer := make([]byte, 64)
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenIntegrityLevel, &buffer[0], uint32(len(buffer)), &size); err != nil {
		t.Fatal(err)
	}
	label := (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buffer[0]))
	return integrityRID(label.Label.Sid)
}

func tokenOwner(t *testing.T, token windows.Token) *windows.SID {
	t.Helper()
	buffer := make([]byte, 128)
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenOwner, &buffer[0], uint32(len(buffer)), &size); err != nil {
		t.Fatal(err)
	}
	owner := (*struct{ Owner *windows.SID })(unsafe.Pointer(&buffer[0]))
	copied, err := owner.Owner.Copy()
	if err != nil {
		t.Fatal(err)
	}
	return copied
}
