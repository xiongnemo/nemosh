package applets

import (
	"bytes"
	"context"
	"math/rand/v2"
	"strings"
	"testing"
)

// withAppletFlag is options with letter given, as the command line would give it.
func withAppletFlag(options appletOptions, letter byte) appletOptions {
	given := map[byte]bool{letter: true}
	for known := range options.given {
		given[known] = true
	}
	options.given = given
	return options
}

// tr with sets of ASCII only reads its input in 64 KiB blocks and maps each byte through a
// table, where it read a rune at a time through a 4 KiB buffer and looked each one up in its set
// in turn: `tr a-z A-Z` of 92 MB took 2.23 s, four times busybox's 0.55 s.
func TestTr_readsInBlocksWhenItsSetsAreASCII(t *testing.T) {
	input := &readCounter{inner: strings.NewReader(strings.Repeat("hello world\n", 1<<20/12))}
	table, err := trSpec(appletOptions{}, []string{"a-z", "A-Z"})
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := table.run(context.Background(), &out, input); err != nil {
		t.Fatal(err)
	}
	if input.reads > 20 {
		t.Errorf("1 MiB took %d reads, want at most 20", input.reads)
	}
	if !strings.HasPrefix(out.String(), "HELLO WORLD\n") {
		t.Errorf("translated %q", out.String()[:24])
	}
}

// The byte table answers exactly as the rune-at-a-time path does, for every option and for input
// that is ASCII, UTF-8, and not UTF-8 at all.
func TestTr_theByteTableAgreesWithTheRunePath(t *testing.T) {
	random := rand.New(rand.NewPCG(1, 2))
	pieces := []string{"a", "b", "z", "A", "Z", " ", "  ", "\r\n", "\t", "é", "中", "\xff", "\xc3", "aa", "eee", "0", "9", "-", "\\"}
	var inputs []string
	for range 200 {
		var input strings.Builder
		for range random.IntN(60) {
			input.WriteString(pieces[random.IntN(len(pieces))])
		}
		inputs = append(inputs, input.String())
	}
	for _, test := range []struct {
		flags    string
		operands []string
	}{
		{"", []string{"a-z", "A-Z"}},
		{"", []string{"a-z", "A"}},
		{"", []string{"abcabc", "xyz"}},
		{"", []string{"[:upper:]", "[:lower:]"}},
		{"", []string{`\r`, `\n`}},
		{"d", []string{"aeiou"}},
		{"d", []string{`\r`}},
		{"s", []string{" "}},
		{"s", []string{"a-z", "A-Z"}},
		{"ds", []string{"a", " "}},
		{"d", []string{"[:space:]"}},
	} {
		options := appletOptions{}
		for _, flag := range test.flags {
			options = withAppletFlag(options, byte(flag))
		}
		table, err := trSpec(options, test.operands)
		if err != nil {
			t.Fatal(err)
		}
		bytesTable, ok := table.byteTable()
		if !ok {
			t.Fatalf("tr -%s %q has ASCII sets and no byte table", test.flags, test.operands)
		}
		for _, input := range inputs {
			var byRune, byByte bytes.Buffer
			if err := table.runRunes(context.Background(), &byRune, strings.NewReader(input)); err != nil {
				t.Fatal(err)
			}
			if err := table.runBytes(context.Background(), &byByte, strings.NewReader(input), bytesTable); err != nil {
				t.Fatal(err)
			}
			if byRune.String() != byByte.String() {
				t.Fatalf("tr -%s %q of %q: runes %q, bytes %q", test.flags, test.operands, input, byRune.String(), byByte.String())
			}
		}
	}
	for _, operands := range [][]string{{"a", "é"}, {"é", "e"}} {
		table, _ := trSpec(appletOptions{}, operands)
		if _, ok := table.byteTable(); ok {
			t.Errorf("tr %q has a byte table though a set is not ASCII", operands)
		}
	}
	complemented, _ := trSpec(withAppletFlag(appletOptions{}, 'c'), []string{"a-z", "_"})
	if _, ok := complemented.byteTable(); ok {
		t.Error("tr -c has a byte table, though its set holds every character but a few")
	}
}
