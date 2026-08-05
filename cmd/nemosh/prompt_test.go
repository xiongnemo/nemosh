package main

import (
	"strings"
	"testing"
)

func TestRenderPrompt_expandsBasicEscapes(t *testing.T) {
	// Given
	values := promptValues{username: "nemo", hostname: "workstation", workingDirectory: "/c/work", symbol: "$"}

	// When
	got := renderPrompt(`\u@\h:\w\n\$ \\`, values)

	// Then
	if want := "nemo@workstation:/c/work\n$ \\"; got != want {
		t.Fatalf("renderPrompt() = %q, want %q", got, want)
	}
}

func TestRenderPrompt_preservesUnknownEscape(t *testing.T) {
	// Given / When
	got := renderPrompt(`left\qright`, promptValues{})

	// Then
	if want := `left\qright`; got != want {
		t.Fatalf("renderPrompt() = %q, want %q", got, want)
	}
}

func TestRenderPrompt_sanitizesInterpolatedControlCharacters(t *testing.T) {
	// Given
	values := promptValues{
		username:         "user\x00\x07\x1b",
		hostname:         "host\r\n\u0085",
		workingDirectory: "dir\t\b\x7f",
		symbol:           string([]byte{'$', 0xff}),
	}

	// When
	got := renderPrompt(`\u|\h|\w|\$`, values)

	// Then
	want := `user\x00\x07\x1b|host\x0d\x0a\x85|dir\x09\x08\x7f|$\xff`
	if got != want {
		t.Fatalf("renderPrompt() = %q, want %q", got, want)
	}
	for _, control := range []string{"\x00", "\x07", "\x08", "\x09", "\x0d", "\x1b", "\x7f", "\u0085"} {
		if strings.Contains(got, control) {
			t.Fatalf("renderPrompt() contains raw control %q in %q", control, got)
		}
	}
}

func TestRenderPrompt_preservesTrustedPromptLiteralControls(t *testing.T) {
	// Given
	template := "\x1b[32m\\u\x1b[0m\\n"
	values := promptValues{username: "nemo\x1b[31m"}

	// When
	got := renderPrompt(template, values)

	// Then
	want := "\x1b[32mnemo\\x1b[31m\x1b[0m\n"
	if got != want {
		t.Fatalf("renderPrompt() = %q, want %q", got, want)
	}
}

func TestRenderPrompt_preservesPrintableUnicodeInInterpolatedValues(t *testing.T) {
	// Given / When
	got := renderPrompt(`\u@\h:\w`, promptValues{
		username:         "用户",
		hostname:         "主机",
		workingDirectory: "/工作/目录",
	})

	// Then
	if want := "用户@主机:/工作/目录"; got != want {
		t.Fatalf("renderPrompt() = %q, want %q", got, want)
	}
}
