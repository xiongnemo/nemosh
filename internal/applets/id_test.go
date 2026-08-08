package applets_test

import (
	"bytes"
	"context"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func runID(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	applet, ok := applets.DefaultRegistry.Lookup("id")
	if !ok {
		t.Fatal("id is not registered")
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: t.TempDir()})
	err := applet.Run(ctx, args, strings.NewReader(""), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

// The default form is busybox-w32's, measured on this machine:
//
//	uid=4095(nemo) gid=4095(nemo) groups=4095(nemo)
func TestID_printsTheBusyBoxForm(t *testing.T) {
	// When
	stdout, stderr, err := runID(t)

	// Then
	if err != nil {
		t.Fatalf("id: %v (stderr %q)", err, stderr)
	}
	shape := regexp.MustCompile(`^uid=\d+\([^)]+\) gid=\d+\([^)]+\) groups=\d+\([^)]+\)\n$`)
	if !shape.MatchString(stdout) {
		t.Fatalf("id = %q, want the busybox form", stdout)
	}
}

func TestID_printsTheRequestedField(t *testing.T) {
	for _, test := range []struct {
		name    string
		args    []string
		numeric bool
	}{
		{name: "user id", args: []string{"-u"}, numeric: true},
		{name: "user name", args: []string{"-un"}},
		{name: "group id", args: []string{"-g"}, numeric: true},
		{name: "group name", args: []string{"-gn"}},
		{name: "group list", args: []string{"-G"}, numeric: true},
		{name: "group names", args: []string{"-Gn"}},
		{name: "clustered the other way", args: []string{"-n", "-u"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			stdout, stderr, err := runID(t, test.args...)

			// Then
			if err != nil {
				t.Fatalf("id %v: %v (stderr %q)", test.args, err, stderr)
			}
			value := strings.TrimSuffix(stdout, "\n")
			if value == "" || strings.Contains(value, "\n") {
				t.Fatalf("id %v = %q, want a single line", test.args, stdout)
			}
			if test.numeric {
				if _, convErr := strconv.Atoi(strings.Fields(value)[0]); convErr != nil {
					t.Fatalf("id %v = %q, want a number", test.args, value)
				}
			}
		})
	}
}

// `id -u` is what a prompt asks to decide between $ and #, so it has to be a
// number a script can compare. busybox-w32 answers 0 only when the process is
// elevated with the Administrators group enabled, and 4095 otherwise.
func TestID_reportsAComparableUserID(t *testing.T) {
	// When
	stdout, _, err := runID(t, "-u")
	if err != nil {
		t.Fatal(err)
	}

	// Then
	uid, convErr := strconv.Atoi(strings.TrimSpace(stdout))
	if convErr != nil {
		t.Fatalf("id -u = %q, want a number", stdout)
	}
	if uid < 0 {
		t.Fatalf("id -u = %d, want it not to be negative", uid)
	}
}

// An option this build does not implement is refused by name, not ignored.
func TestID_refusesAnUnknownOption(t *testing.T) {
	// When
	stdout, stderr, err := runID(t, "-Z")

	// Then
	if err == nil {
		t.Fatal("id -Z succeeded, want a refusal")
	}
	if stdout != "" {
		t.Fatalf("id -Z wrote %q before refusing", stdout)
	}
	if message := stderr + err.Error(); !strings.Contains(message, "Z") {
		t.Fatalf("id -Z reported %q, want it to name the option", message)
	}
}

// -n without a selector is a usage error in coreutils and busybox both.
func TestID_refusesNameWithoutASelector(t *testing.T) {
	// When
	_, stderr, err := runID(t, "-n")

	// Then
	if err == nil {
		t.Fatal("id -n succeeded, want a refusal")
	}
	if message := stderr + err.Error(); !strings.Contains(message, "-n") {
		t.Fatalf("id -n reported %q, want it to explain", message)
	}
}
