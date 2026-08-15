package main

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// The ssh_config grammar, as far as this reads it.
func TestSSHConfigHostNames(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{name: "one host", line: "Host build", want: []string{"build"}},
		{name: "several on a line", line: "Host build prod", want: []string{"build", "prod"}},
		{
			// HostName is read as well as Host. They answer different halves of
			// the same question: Host is the short alias someone wrote the file
			// to have, HostName is what it resolves to and is also typed. fish
			// reads only Host; bash-completion reads both, and is right.
			name: "the real name too", line: "  HostName build.example.com", want: []string{"build.example.com"},
		},
		{name: "indented and mixed case", line: "\thost Build", want: []string{"Build"}},
		{name: "the equals spelling ssh_config allows", line: "Host=build", want: []string{"build"}},
		{
			// A pattern configures a set. Completing it would put a name on the
			// line that resolves to nothing.
			name: "a wildcard is a rule, not a machine", line: "Host *", want: nil,
		},
		{name: "a partial wildcard likewise", line: "Host prod-*", want: nil},
		{name: "a negation likewise", line: "Host !secret", want: nil},
		{
			// ssh's own substitution token, which appears in HostName lines.
			name: "a substitution token", line: "HostName %h.internal", want: nil,
		},
		{name: "another keyword", line: "  User nemo", want: nil},
		{name: "a comment", line: "# Host commented", want: nil},
		{name: "nothing", line: "   ", want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := hostNamesInLine(test.line, true)

			// Then
			if !slices.Equal(got, test.want) {
				t.Fatalf("hostNamesInLine(%q) = %v, want %v", test.line, got, test.want)
			}
		})
	}
}

func TestHostsFileNames(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{name: "a name and its aliases", line: "127.0.0.1 localhost lh", want: []string{"localhost", "lh"}},
		{name: "a trailing comment", line: "10.0.0.5 build # the build box", want: []string{"build"}},
		{
			// 0.0.0.0 is the convention for "this name goes nowhere", which is
			// what an ad-blocking hosts file is made of. A name installed in
			// order *not* to reach is not somewhere to connect.
			name: "a blocked name", line: "0.0.0.0 ads.example.com", want: nil,
		},
		{
			// Left alone: that is a real local service as often as it is a block.
			name: "loopback is not treated as a block", line: "127.0.0.1 dev.local", want: []string{"dev.local"},
		},
		{name: "an address with no name", line: "10.0.0.5", want: nil},
		{name: "a comment", line: "# 10.0.0.5 nope", want: nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got := hostNamesInLine(test.line, false)

			// Then
			if !slices.Equal(got, test.want) {
				t.Fatalf("hostNamesInLine(%q) = %v, want %v", test.line, got, test.want)
			}
		})
	}
}

// Editing ~/.ssh/config and using the new name straight away is the ordinary
// loop, so the index has to notice. It compares size and modification time,
// which is what stat already knows -- one syscall per prompt, none per
// keystroke.
func TestHostIndex_rebuildsWhenTheConfigChanges(t *testing.T) {
	// Given
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".ssh"), 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(home, ".ssh", "config")
	if err := os.WriteFile(path, []byte("Host first\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	index := newHostIndex()
	index.refresh([]string{path})
	waitForHostIndex(t, index, []string{"first"})

	// When
	if err := os.WriteFile(path, []byte("Host first\nHost second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	index.refresh([]string{path})

	// Then
	waitForHostIndex(t, index, []string{"first", "second"})
}

// A machine with no ~/.ssh/config at all is the common case on a fresh install,
// and it must answer "nothing" rather than never finishing -- the index being
// unbuilt and the index being empty look the same to a caller otherwise.
func TestHostIndex_settlesWithNoSourcesAtAll(t *testing.T) {
	// Given
	index := newHostIndex()

	// When
	index.refresh([]string{filepath.Join(t.TempDir(), "nothing-here")})

	// Then
	waitForHostIndex(t, index, nil)
}

func waitForHostIndex(t *testing.T, index *hostIndex, want []string) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for {
		if got := index.candidates(); slices.Equal(got, want) && index.builtFrom() != "" {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("host index = %v, want %v", index.candidates(), want)
		}
		time.Sleep(time.Millisecond)
	}
}
