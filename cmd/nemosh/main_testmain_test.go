package main

import (
	"os"
	"strings"
	"testing"
)

// TestMain removes $ENV for the whole package.
//
// An interactive shell sources it, which is correct in production and wrong in
// a test: $ENV is a machine setting, so a test that reads it passes or fails
// depending on whose machine it runs on. This developer's points at a busybox
// `.ashrc` that sets a coloured prompt and aliases, which is exactly the kind
// of thing prompt assertions must not see.
//
// A test that wants a startup file sets one explicitly, as startup_test.go does.
//
// Unsetting ENV stopped being enough on 2026-11-15, when a shell with no ENV came to read
// ~/.nemoshrc: from then every -i test ran the developer's own rc file. So the run has a home
// of its own as well; see isolateHome.
//
// The same for %APPDATA%, where releases up to 1.5 kept completion specs: a session says so when
// one is there, so a prompt assertion would pass or fail by whose machine it ran on, and a test
// would be looking in a real home. The session reads its environment through sessionSpecLookup,
// and here that has no APPDATA; spec_dirs_test.go hands its own. APPDATA itself is left alone,
// since the go commands some tests run keep their settings under it.
func TestMain(m *testing.M) {
	if err := os.Unsetenv("ENV"); err != nil {
		panic(err)
	}
	home, owned := isolateHome()
	sessionSpecLookup = func(name string) (string, bool) {
		if name == "APPDATA" {
			return "", false
		}
		return os.LookupEnv(name)
	}
	status := m.Run()
	if owned {
		os.RemoveAll(home)
	}
	os.Exit(status)
}

// withoutANSI strips colour so a prompt assertion can be about what the prompt
// says rather than how it is painted. The default prompt gained colour, and an
// assertion on its literal bytes would otherwise have to be rewritten every
// time a sequence changes.
func withoutANSI(text string) string {
	var plain strings.Builder
	for index := 0; index < len(text); index++ {
		if text[index] != 0x1b {
			plain.WriteByte(text[index])
			continue
		}
		// CSI runs to its final byte in @ through ~.
		index++
		if index < len(text) && text[index] == '[' {
			for index++; index < len(text); index++ {
				if text[index] >= '@' && text[index] <= '~' {
					break
				}
			}
		}
	}
	return plain.String()
}
