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
func TestMain(m *testing.M) {
	if err := os.Unsetenv("ENV"); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
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
