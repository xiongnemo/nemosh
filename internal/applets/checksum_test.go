package applets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The rest of the checksum family.
//
// nemosh had md5sum and sha256sum. busybox-w32 also has sha1sum, sha384sum,
// sha512sum, sha3sum, cksum, crc32 and sum, and a clean Windows machine has none
// of them -- which is most of why anyone reaches for this after a download.
//
// Every digest below was measured against busybox-w32 v1.38.0 on 2026-08-22 for
// the six bytes "hello\n", and the SHA-3 values cross-checked against Go's
// crypto/sha3.

func runChecksum(t *testing.T, dir string, applet string, stdin string, args ...string) (string, string, error) {
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

func checksumFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "h.txt"), []byte("hello\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "e.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestChecksum_theHashFamily(t *testing.T) {
	for _, test := range []struct {
		applet string
		want   string
	}{
		{applet: "md5sum", want: "b1946ac92492d2347c6235b4d2611184"},
		{applet: "sha1sum", want: "f572d396fae9206628714fb2ce00f72e94f2258f"},
		{applet: "sha256sum", want: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"},
		{applet: "sha384sum", want: "1d0f284efe3edea4b9ca3bd514fa134b17eae361ccc7a1eefeff801b9bd6604e01f21f6bf249ef030599f0c218f2ba8c"},
		{applet: "sha512sum", want: "e7c22b994c59d9cf2b48e549b1e24666636045930d3da7c1acb299d1c3b7f931f94aae41edda2c2b207a36e10f8bcb8d45223e54878f5b316e7ce3b6bc019629"},
		// busybox's sha3sum defaults to 224 bits, not 512. Measured.
		{applet: "sha3sum", want: "5093b1ea1fed43f347b4bf8f8e61334e751516506e390b0fa67758d3"},
	} {
		t.Run(test.applet, func(t *testing.T) {
			dir := checksumFixture(t)
			stdout, stderr, err := runChecksum(t, dir, test.applet, "", "h.txt")
			if err != nil {
				t.Fatalf("%s: %v (%s)", test.applet, err, stderr)
			}
			// GNU's two spaces between digest and name.
			if want := test.want + "  h.txt\n"; stdout != want {
				t.Fatalf("%s = %q, want %q", test.applet, stdout, want)
			}
			// Stdin is named `-`, as it is for the two that already existed.
			stdout, _, err = runChecksum(t, dir, test.applet, "hello\n")
			if err != nil {
				t.Fatal(err)
			}
			if want := test.want + "  -\n"; stdout != want {
				t.Fatalf("%s from stdin = %q, want %q", test.applet, stdout, want)
			}
		})
	}
}

// -a picks the SHA-3 width. All four were cross-checked against Go's crypto/sha3
// as well as against busybox, because a wrong constant here produces a
// plausible-looking digest that is silently useless.
func TestChecksum_sha3Widths(t *testing.T) {
	dir := checksumFixture(t)
	for _, test := range []struct {
		bits string
		want string
	}{
		{bits: "224", want: "5093b1ea1fed43f347b4bf8f8e61334e751516506e390b0fa67758d3"},
		{bits: "256", want: "b314e28493eae9dab57ac4f0c6d887bddbbeb810e900d818395ace558e96516d"},
		{bits: "384", want: "459b2844fea6e3a937a8397c0d69c06d9c6c943e155da454c638f5424296e994fd0339ea234367ff014493b51adb9d2e"},
		{bits: "512", want: "ac766ba623301e0ad63c48cb2fc469d10145f65c9f1f28fe761c78c386ed295a1fda1b05e280354e620757d8a83e05a45f66438dd734278668c1c27ac6f27150"},
	} {
		stdout, stderr, err := runChecksum(t, dir, "sha3sum", "", "-a", test.bits, "h.txt")
		if err != nil {
			t.Fatalf("sha3sum -a %s: %v (%s)", test.bits, err, stderr)
		}
		if want := test.want + "  h.txt\n"; stdout != want {
			t.Fatalf("sha3sum -a %s = %q, want %q", test.bits, stdout, want)
		}
	}
	// A width SHA-3 does not have is refused rather than rounded to one it does.
	if _, _, err := runChecksum(t, dir, "sha3sum", "", "-a", "300", "h.txt"); err == nil {
		t.Fatal("sha3sum -a 300 succeeded, want a refusal")
	}
}

// cksum is the POSIX CRC, which is not the same CRC-32 as `crc32`: it is
// MSB-first over a different polynomial, it feeds the file *length* through the
// register afterwards, and it complements the result. Its output carries the size.
func TestChecksum_cksum(t *testing.T) {
	dir := checksumFixture(t)
	stdout, stderr, err := runChecksum(t, dir, "cksum", "", "h.txt")
	if err != nil {
		t.Fatalf("cksum: %v (%s)", err, stderr)
	}
	if want := "3015617425 6 h.txt\n"; stdout != want {
		t.Fatalf("cksum = %q, want %q", stdout, want)
	}
	// An empty file is the all-ones register, which is the clearest check that
	// the final complement is there.
	stdout, _, err = runChecksum(t, dir, "cksum", "", "e.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "4294967295 0 e.txt\n"; stdout != want {
		t.Fatalf("cksum on an empty file = %q, want %q", stdout, want)
	}
	// Stdin has no name to print, so the line is the sum and the size.
	stdout, _, err = runChecksum(t, dir, "cksum", "hello\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "3015617425 6\n"; stdout != want {
		t.Fatalf("cksum from stdin = %q, want %q", stdout, want)
	}
}

// crc32 is the ordinary IEEE CRC-32, eight lowercase hex digits.
func TestChecksum_crc32(t *testing.T) {
	dir := checksumFixture(t)
	for _, test := range []struct {
		file string
		want string
	}{
		{file: "h.txt", want: "363a3020"},
		{file: "e.txt", want: "00000000"},
	} {
		stdout, stderr, err := runChecksum(t, dir, "crc32", "", test.file)
		if err != nil {
			t.Fatalf("crc32 %s: %v (%s)", test.file, err, stderr)
		}
		if want := test.want + " " + test.file + "\n"; stdout != want {
			t.Fatalf("crc32 %s = %q, want %q", test.file, stdout, want)
		}
	}
	stdout, _, err := runChecksum(t, dir, "crc32", "hello\n")
	if err != nil {
		t.Fatal(err)
	}
	if want := "363a3020\n"; stdout != want {
		t.Fatalf("crc32 from stdin = %q, want %q", stdout, want)
	}
}

// sum is the historical 16-bit checksum, in two incompatible flavours: BSD
// rotates the accumulator and counts 1024-byte blocks, System V folds a plain
// byte sum and counts 512-byte blocks. They disagree on the same file, which is
// why -s exists and why both are pinned.
func TestChecksum_sum(t *testing.T) {
	dir := checksumFixture(t)
	for _, test := range []struct {
		name string
		args []string
		want string
	}{
		// The name is omitted for a single operand, which is what GNU does.
		{name: "bsd by default", args: []string{"h.txt"}, want: "36979     1\n"},
		{name: "-r is the default spelled out", args: []string{"-r", "h.txt"}, want: "36979     1\n"},
		{name: "system v", args: []string{"-s", "h.txt"}, want: "542 1 h.txt\n"},
		{name: "an empty file", args: []string{"e.txt"}, want: "00000     0\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stdout, stderr, err := runChecksum(t, dir, "sum", "", test.args...)
			if err != nil {
				t.Fatalf("sum %v: %v (%s)", test.args, err, stderr)
			}
			if stdout != test.want {
				t.Fatalf("sum %v = %q, want %q", test.args, stdout, test.want)
			}
		})
	}
	// With more than one operand the name is printed, because otherwise the
	// lines could not be told apart.
	stdout, _, err := runChecksum(t, dir, "sum", "", "h.txt", "e.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := "36979     1 h.txt\n00000     0 e.txt\n"; stdout != want {
		t.Fatalf("sum over two files = %q, want %q", stdout, want)
	}
}

// -c verifies a list, and it has to work for every member of the family rather
// than only the two that had it.
func TestChecksum_checkModeForANewHash(t *testing.T) {
	dir := checksumFixture(t)
	sums := "f572d396fae9206628714fb2ce00f72e94f2258f  h.txt\n"
	if err := os.WriteFile(filepath.Join(dir, "SHA1SUMS"), []byte(sums), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, stderr, err := runChecksum(t, dir, "sha1sum", "", "-c", "SHA1SUMS")
	if err != nil {
		t.Fatalf("sha1sum -c: %v (%s)", err, stderr)
	}
	if want := "h.txt: OK\n"; stdout != want {
		t.Fatalf("sha1sum -c = %q, want %q", stdout, want)
	}

	bad := "0000000000000000000000000000000000000000  h.txt\n"
	if err := os.WriteFile(filepath.Join(dir, "BAD"), []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err = runChecksum(t, dir, "sha1sum", "", "-c", "BAD")
	if err == nil {
		t.Fatal("sha1sum -c on a wrong digest succeeded, want a failure")
	}
	if !strings.Contains(stdout, "FAILED") {
		t.Fatalf("sha1sum -c on a wrong digest said %q, want FAILED", stdout)
	}
}

// A lone `-` is stdin here too, and the family shares one operand seam.
func TestChecksum_dashIsStdin(t *testing.T) {
	dir := checksumFixture(t)
	for _, applet := range []string{"sha1sum", "sha512sum", "sha3sum", "cksum", "crc32", "sum"} {
		if _, stderr, err := runChecksum(t, dir, applet, "hello\n", "-"); err != nil {
			t.Fatalf("%s -: %v (%s)", applet, err, stderr)
		}
	}
}
