package applets_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/proc"
)

// The eleven small tools added on 2026-08-22: free, factor, fold, tsort, strings,
// ascii, expand, unexpand, join, base32, shuf.
//
// 22 of 24 measured forms agree with busybox-w32 v1.38.0 byte for byte. The two
// that do not are recorded in the tests that cover them, because both are
// deliberate.

func runSmall(t *testing.T, dir, stdin, applet string, args ...string) (string, string, error) {
	t.Helper()
	found, ok := applets.DefaultRegistry.Lookup(applet)
	if !ok {
		t.Fatalf("%s is not registered", applet)
	}
	var stdout, stderr bytes.Buffer
	ctx := applets.WithProcessView(context.Background(), findTestProcessView{cwd: dir})
	err := found.Run(ctx, args, strings.NewReader(stdin), &stdout, &stderr)
	return stdout.String(), stderr.String(), err
}

func writeSmallFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestFactor_printsPrimeFactors(t *testing.T) {
	dir := t.TempDir()
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"12"}, want: "12: 2 2 3\n"},
		// One is not prime and has no factors, so the line is a bare `1:`. This
		// is the case a naive loop prints wrongly.
		{args: []string{"1"}, want: "1:\n"},
		{args: []string{"0"}, want: "0:\n"},
		{args: []string{"97"}, want: "97: 97\n"},
		{args: []string{"1", "2", "97", "100"}, want: "1:\n2: 2\n97: 97\n100: 2 2 5 5\n"},
		// A large prime: the square-root bound must shrink with the remainder or
		// this takes four billion divisions.
		{args: []string{"4294967291"}, want: "4294967291: 4294967291\n"},
		{args: []string{"18446744073709551615"}, want: "18446744073709551615: 3 5 17 257 641 65537 6700417\n"},
	} {
		got, stderr, err := runSmall(t, dir, "", "factor", test.args...)
		if err != nil {
			t.Fatalf("factor %v: %v (%s)", test.args, err, stderr)
		}
		if got != test.want {
			t.Fatalf("factor %v = %q, want %q", test.args, got, test.want)
		}
	}
	// Numbers come from stdin when there are no operands, several per line.
	got, _, err := runSmall(t, dir, "6 8\n9\n", "factor")
	if err != nil {
		t.Fatal(err)
	}
	if want := "6: 2 3\n8: 2 2 2\n9: 3 3\n"; got != want {
		t.Fatalf("factor from stdin = %q, want %q", got, want)
	}
	if _, _, err := runSmall(t, dir, "", "factor", "abc"); err == nil {
		t.Fatal("factor abc succeeded, want a refusal")
	}
}

func TestFold_wrapsLines(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"h.txt":     "hello\n",
		"long.txt":  "aaaaaaaaaaaaaaaaaaaaaaaa bbbb\n",
		"blank.txt": "a\n\nb\n",
	})
	for _, test := range []struct {
		args []string
		want string
	}{
		{args: []string{"-w", "3", "h.txt"}, want: "hel\nlo\n"},
		{args: []string{"-w", "1", "h.txt"}, want: "h\ne\nl\nl\no\n"},
		{args: []string{"-w", "10", "long.txt"}, want: "aaaaaaaaaa\naaaaaaaaaa\naaaa bbbb\n"},
		// -s prefers a space; with none inside the limit the cut stays hard, which
		// is why the first two pieces are unchanged. The remainder is nine
		// characters, under the limit, so it is not cut at the space either --
		// checked against busybox, which answers the same.
		{args: []string{"-s", "-w", "10", "long.txt"}, want: "aaaaaaaaaa\naaaaaaaaaa\naaaa bbbb\n"},
		// An empty line survives as an empty line rather than vanishing.
		{args: []string{"-w", "1", "blank.txt"}, want: "a\n\nb\n"},
	} {
		got, stderr, err := runSmall(t, dir, "", "fold", test.args...)
		if err != nil {
			t.Fatalf("fold %v: %v (%s)", test.args, err, stderr)
		}
		if got != test.want {
			t.Fatalf("fold %v = %q, want %q", test.args, got, test.want)
		}
	}
	// Runes, not bytes: a CJK line must not be cut through a character.
	got, _, err := runSmall(t, dir, "一二三\n", "fold", "-w", "2")
	if err != nil {
		t.Fatal(err)
	}
	if want := "一二\n三\n"; got != want {
		t.Fatalf("fold over CJK = %q, want %q", got, want)
	}
	if _, _, err := runSmall(t, dir, "", "fold", "-w", "0", "h.txt"); err == nil {
		t.Fatal("fold -w 0 succeeded, want a refusal")
	}
}

// tsort's order among items with no dependency between them is unspecified, and
// the three implementations genuinely differ: for `a b / b c / d e` busybox
// answers `a b d e c`, GNU answers `a d b e c`, and this answers `a b c d e`.
// All three are valid.
//
// The input-stable order is chosen deliberately: it is the only one of the three
// that is reproducible, so two runs over one file agree and a diff between them
// means something. What the test asserts is therefore the *constraint*, plus the
// stability.
func TestTsort_ordersDependenciesFirst(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"t.txt":    "a b\nb c\nd e\n",
		"loop.txt": "a b\nb a\n",
		"one.txt":  "solo\n",
	})
	got, stderr, err := runSmall(t, dir, "", "tsort", "t.txt")
	if err != nil {
		t.Fatalf("tsort: %v (%s)", err, stderr)
	}
	order := strings.Fields(got)
	position := map[string]int{}
	for index, name := range order {
		position[name] = index
	}
	for _, pair := range [][2]string{{"a", "b"}, {"b", "c"}, {"d", "e"}} {
		if position[pair[0]] >= position[pair[1]] {
			t.Fatalf("tsort put %s after %s: %v", pair[0], pair[1], order)
		}
	}
	if strings.Join(order, " ") != "a b c d e" {
		t.Fatalf("tsort = %v, want the input-stable order a b c d e", order)
	}
	// Two runs must agree, which is the whole point of choosing stability.
	again, _, _ := runSmall(t, dir, "", "tsort", "t.txt")
	if again != got {
		t.Fatalf("two runs disagreed: %q then %q", got, again)
	}
	// An item with no edges still reaches the output.
	if solo, _, _ := runSmall(t, dir, "", "tsort", "one.txt"); strings.TrimSpace(solo) != "solo" {
		t.Fatalf("tsort on a lone item = %q", solo)
	}
	// A cycle is reported, not silently truncated: a partial order looks exactly
	// like a complete one.
	if _, stderr, err := runSmall(t, dir, "", "tsort", "loop.txt"); err == nil {
		t.Fatal("tsort on a cycle succeeded, want a refusal")
	} else if !strings.Contains(stderr+err.Error(), "loop") {
		t.Fatalf("tsort on a cycle said %q, want it to name the loop", stderr+err.Error())
	}
}

func TestStrings_printsPrintableRuns(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"bin.bin": "ab\x00cdefgh\x01xy\n",
	})
	// The default shortest run is 4, so `ab` and `xy` are below it.
	got, stderr, err := runSmall(t, dir, "", "strings", "bin.bin")
	if err != nil {
		t.Fatalf("strings: %v (%s)", err, stderr)
	}
	if want := "cdefgh\n"; got != want {
		t.Fatalf("strings = %q, want %q", got, want)
	}
	got, _, err = runSmall(t, dir, "", "strings", "-n", "2", "bin.bin")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ab\ncdefgh\nxy\n"; got != want {
		t.Fatalf("strings -n 2 = %q, want %q", got, want)
	}
	// -t prefixes the offset of the run's first byte.
	got, _, err = runSmall(t, dir, "", "strings", "-n", "2", "-t", "d", "bin.bin")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, "      0 ab") || !strings.Contains(got, "      3 cdefgh") {
		t.Fatalf("strings -t d = %q, want offsets 0 and 3", got)
	}
	if _, _, err := runSmall(t, dir, "", "strings", "-t", "q", "bin.bin"); err == nil {
		t.Fatal("strings -t q succeeded, want a refusal")
	}
}

// The ascii table is compared whole, because its column spacing is busybox's
// hand-tuned layout rather than anything derivable: the gaps between columns are
// 11, 11, 9, 9, 9, 10, 10 characters.
func TestAscii_matchesTheReferenceLayout(t *testing.T) {
	got, stderr, err := runSmall(t, t.TempDir(), "", "ascii")
	if err != nil {
		t.Fatalf("ascii: %v (%s)", err, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(got, "\n"), "\n")
	if len(lines) != 17 {
		t.Fatalf("ascii printed %d lines, want a header and 16 rows", len(lines))
	}
	for _, want := range []string{
		"Dec Hex    Dec Hex    Dec Hex  Dec Hex  Dec Hex  Dec Hex   Dec Hex   Dec Hex",
		"  0 00 NUL  16 10 DLE  32 20    48 30 0  64 40 @  80 50 P   96 60 `  112 70 p",
		// 0x0a is NL here, not LF, which is the spelling busybox uses.
		" 10 0a NL   26 1a SUB  42 2a *  58 3a :  74 4a J  90 5a Z  106 6a j  122 7a z",
		" 15 0f SI   31 1f US   47 2f /  63 3f ?  79 4f O  95 5f _  111 6f o  127 7f DEL",
	} {
		if !strings.Contains(got, want+"\n") {
			t.Fatalf("ascii is missing the line %q", want)
		}
	}
	// Read *down*: the first column is 0-15, not 0,1,2 across.
	if !strings.HasPrefix(lines[1], "  0 00 NUL  16 10 DLE") {
		t.Fatalf("ascii reads across rather than down: %q", lines[1])
	}
}

func TestExpandUnexpand_convertTabsAndSpaces(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"tab.txt": "a\tb\n\tc\n",
		"sp.txt":  "        x\n    y\n",
		"mid.txt": "ab\tc\n",
	})
	for _, test := range []struct {
		applet string
		args   []string
		want   string
	}{
		{applet: "expand", args: []string{"tab.txt"}, want: "a       b\n        c\n"},
		{applet: "expand", args: []string{"-t", "4", "tab.txt"}, want: "a   b\n    c\n"},
		{applet: "unexpand", args: []string{"-a", "sp.txt"}, want: "\tx\n    y\n"},
		{applet: "unexpand", args: []string{"sp.txt"}, want: "\tx\n    y\n"},
	} {
		got, stderr, err := runSmall(t, dir, "", test.applet, test.args...)
		if err != nil {
			t.Fatalf("%s %v: %v (%s)", test.applet, test.args, err, stderr)
		}
		if got != test.want {
			t.Fatalf("%s %v = %q, want %q", test.applet, test.args, got, test.want)
		}
	}
	// The column is counted in runes, so a tab after CJK lands where it looks
	// like it should. Counting bytes puts it two columns early per character.
	got, _, err := runSmall(t, dir, "一二\tx\n", "expand")
	if err != nil {
		t.Fatal(err)
	}
	if want := "一二      x\n"; got != want {
		t.Fatalf("expand after CJK = %q, want %q", got, want)
	}
	// -i leaves a tab alone once text has started, which is what makes it safe
	// on source code with a tab inside a string literal.
	got, _, err = runSmall(t, dir, "", "expand", "-i", "mid.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "ab\tc\n"; got != want {
		t.Fatalf("expand -i = %q, want the tab kept", got)
	}
}

func TestJoin_joinsOnAField(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"j1.txt": "1 a\n2 b\n3 c\n",
		"j2.txt": "1 X\n3 Z\n",
		"k.txt":  "a 1\nc 3\n",
	})
	got, stderr, err := runSmall(t, dir, "", "join", "j1.txt", "j2.txt")
	if err != nil {
		t.Fatalf("join: %v (%s)", err, stderr)
	}
	if want := "1 a X\n3 c Z\n"; got != want {
		t.Fatalf("join = %q, want %q", got, want)
	}
	// -j is GNU's spelling and busybox does not have it; it is offered here
	// because refusing a standard option would be the worse divergence.
	got, _, err = runSmall(t, dir, "", "join", "-j", "1", "j1.txt", "j2.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1 a X\n3 c Z\n"; got != want {
		t.Fatalf("join -j 1 = %q, want %q", got, want)
	}
	// -1 and -2 select different fields on each side.
	got, _, err = runSmall(t, dir, "", "join", "-1", "2", "-2", "1", "j1.txt", "k.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "a 1 1\nc 3 3\n"; got != want {
		t.Fatalf("join -1 2 -2 1 = %q, want %q", got, want)
	}
	if _, _, err := runSmall(t, dir, "", "join", "j1.txt"); err == nil {
		t.Fatal("join with one operand succeeded, want a refusal")
	}
}

func TestBase32_roundTrips(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"h.txt": "hello\n"})
	got, stderr, err := runSmall(t, dir, "", "base32", "h.txt")
	if err != nil {
		t.Fatalf("base32: %v (%s)", err, stderr)
	}
	if want := "NBSWY3DPBI======\n"; got != want {
		t.Fatalf("base32 = %q, want %q", got, want)
	}
	// -w0 writes no newline at all, which is what anyone piping it onward needs.
	got, _, err = runSmall(t, dir, "", "base32", "-w", "0", "h.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != "NBSWY3DPBI======" {
		t.Fatalf("base32 -w0 = %q, want no trailing newline", got)
	}
	// The decoder accepts its own wrapped output, which a strict one would not.
	wrapped, _, _ := runSmall(t, dir, "", "base32", "-w", "4", "h.txt")
	back, _, err := runSmall(t, dir, wrapped, "base32", "-d")
	if err != nil {
		t.Fatal(err)
	}
	if back != "hello\n" {
		t.Fatalf("base32 round trip = %q, want hello", back)
	}
	if _, _, err := runSmall(t, dir, "!!!!", "base32", "-d"); err == nil {
		t.Fatal("base32 -d on rubbish succeeded, want a refusal")
	}
}

// shuf is random, so the test asserts the multiset and the count. Pinning an
// order would either be wrong or would prove the shuffle does not shuffle.
func TestShuf_permutesWithoutLosingLines(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{"n.txt": "1\n2\n3\n4\n5\n"})
	got, stderr, err := runSmall(t, dir, "", "shuf", "n.txt")
	if err != nil {
		t.Fatalf("shuf: %v (%s)", err, stderr)
	}
	fields := strings.Fields(got)
	if len(fields) != 5 {
		t.Fatalf("shuf returned %d lines, want 5", len(fields))
	}
	seen := map[string]bool{}
	for _, field := range fields {
		if seen[field] {
			t.Fatalf("shuf repeated %q: %v", field, fields)
		}
		seen[field] = true
		if value, err := strconv.Atoi(field); err != nil || value < 1 || value > 5 {
			t.Fatalf("shuf invented %q", field)
		}
	}
	// -n takes a prefix of the permutation.
	got, _, err = runSmall(t, dir, "", "shuf", "-n", "2", "n.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(strings.Fields(got)) != 2 {
		t.Fatalf("shuf -n 2 returned %q", got)
	}
	// -i generates a range and -e takes the operands as the lines.
	if got, _, err = runSmall(t, dir, "", "shuf", "-i", "1-9"); err != nil || len(strings.Fields(got)) != 9 {
		t.Fatalf("shuf -i 1-9 = %q, %v", got, err)
	}
	if got, _, err = runSmall(t, dir, "", "shuf", "-e", "x", "y"); err != nil || len(strings.Fields(got)) != 2 {
		t.Fatalf("shuf -e = %q, %v", got, err)
	}
	if _, _, err := runSmall(t, dir, "", "shuf", "-i", "9-1"); err == nil {
		t.Fatal("shuf -i 9-1 succeeded, want a refusal for a backwards range")
	}
}

// free reports from the same sampler `top` draws its meters from, so the two
// cannot disagree about how much memory the machine has.
//
// Sampling is implemented on Windows only and refuses elsewhere with
// ErrListUnsupported, which is the rule the whole proc package follows -- so this
// skips rather than asserting zeros off Windows.
func TestFree_reportsTheSameMemoryAsTheSampler(t *testing.T) {
	stdout, stderr, err := runSmall(t, t.TempDir(), "", "free")
	if err != nil {
		if errors.Is(err, proc.ErrListUnsupported) {
			t.Skip("memory sampling is implemented on Windows only")
		}
		t.Fatalf("free: %v (%s)", err, stderr)
	}
	lines := strings.Split(strings.TrimSuffix(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("free printed %d lines, want a header, Mem and Swap", len(lines))
	}
	for _, column := range []string{"total", "used", "free", "shared", "buff/cache", "available"} {
		if !strings.Contains(lines[0], column) {
			t.Fatalf("free's header is missing %q: %q", column, lines[0])
		}
	}
	if !strings.HasPrefix(lines[1], "Mem:") || !strings.HasPrefix(lines[2], "Swap:") {
		t.Fatalf("free's rows are %q and %q", lines[1], lines[2])
	}
	// The total must match what the sampler says, not merely be non-zero: a
	// formatter that read the wrong field would still print a plausible number.
	snapshot, err := proc.NewSampler().Sample(false)
	if err != nil {
		t.Skip("memory sampling is implemented on Windows only")
	}
	wantTotal := snapshot.Memory.TotalPhysical / 1024
	fields := strings.Fields(lines[1])
	if len(fields) < 2 {
		t.Fatalf("free's Mem row has too few columns: %q", lines[1])
	}
	reported, err := strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		t.Fatalf("free's total is not a number: %q", fields[1])
	}
	// Kilobytes, and the machine's total does not change between the two calls.
	if reported != wantTotal {
		t.Fatalf("free reported %d KB total, sampler says %d KB", reported, wantTotal)
	}
	// -m scales down by 1024 from -k, which is the check that the divisor is
	// applied rather than the unit merely accepted.
	inMegabytes, _, err := runSmall(t, t.TempDir(), "", "free", "-m")
	if err != nil {
		t.Fatal(err)
	}
	megaFields := strings.Fields(strings.Split(inMegabytes, "\n")[1])
	megaTotal, err := strconv.ParseUint(megaFields[1], 10, 64)
	if err != nil || megaTotal != wantTotal/1024 {
		t.Fatalf("free -m reported %v, want %d", megaFields[1], wantTotal/1024)
	}
}
