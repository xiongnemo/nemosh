package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
)

// isolateHome gives the run a home of its own, and answers where, for TestMain to remove.
//
// No test reads the rc file of whoever runs the suite, or writes their history. A session
// sources ~/.nemoshrc and saves its lines to ~/.nemosh_history, and HOME, where it is not set,
// is %USERPROFILE%, as busybox-w32 makes it -- so every -i test ran the developer's own rc
// file, and a test's prompt was the one they had drawn for themselves.
//
// USERPROFILE names the new home, and HOME, HISTFILE and the XDG variables, any of which could
// name a file elsewhere, are unset. A test that wants one sets it. Go's module cache is found
// from the profile too, and three tests here run the go tool, so GOPATH and GOMODCACHE are
// pinned first to what `go env` says they are.
//
// A helper process, the test binary run again by a test, has its parent's home, which
// NEMOSH_TEST_HOME names, and does not remove it: fifteen helpers each made one of their own,
// and left it behind, in the first run.
func isolateHome() (home string, owned bool) {
	if home := os.Getenv("NEMOSH_TEST_HOME"); home != "" {
		return home, false
	}
	if output, err := exec.Command("go", "env", "GOPATH", "GOMODCACHE").Output(); err == nil {
		values := strings.Split(strings.TrimSpace(string(output)), "\n")
		for index, name := range []string{"GOPATH", "GOMODCACHE"} {
			if index < len(values) && strings.TrimSpace(values[index]) != "" {
				os.Setenv(name, strings.TrimSpace(values[index]))
			}
		}
	}
	home, err := os.MkdirTemp("", "nemosh-test-home-")
	if err != nil {
		panic(err)
	}
	os.Setenv("NEMOSH_TEST_HOME", home)
	os.Setenv("USERPROFILE", home)
	for _, name := range []string{"HOME", "HISTFILE", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"} {
		os.Unsetenv(name)
	}
	return home, true
}

func TestMain_givesTheRunAHomeOfItsOwn(t *testing.T) {
	// Given
	var stdout bytes.Buffer
	cmd := command{stdin: &bytes.Buffer{}, stdout: &stdout, stderr: &bytes.Buffer{}}

	// When
	err := cmd.run(context.Background(), []string{"nemosh", "-c", `echo "$HOME"; echo "${ENV-unset} ${HISTFILE-unset}"`})

	// Then
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) != 2 || !strings.HasPrefix(path.Base(lines[0]), "nemosh-test-home-") || lines[1] != "unset unset" {
		t.Fatalf("stdout = %q, want HOME a directory of the run's own, and ENV and HISTFILE unset", stdout.String())
	}
}
