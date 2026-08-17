package applets_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Expectations measured from GNU coreutils on the machine this was written on.
func TestBase64(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "encodes", stdin: "hello", want: "aGVsbG8=\n"},
		{name: "decodes", args: []string{"-d"}, stdin: "aGVsbG8=\n", want: "hello"},
		{
			// Not politeness but necessity: the encoder wraps at 76 columns, so
			// its own output is not valid base64 to a strict decoder. GNU accepts
			// its own wrapped output, and so must this.
			name: "decoding ignores the newlines encoding added",
			args: []string{"-d"}, stdin: "aGVs\nbG8=\n", want: "hello",
		},
		{name: "empty input encodes to nothing", stdin: "", want: ""},
		{
			// -w0 emits no newline at all, which is what anyone piping the output
			// onwards wants. Measured: `base64 -w0 | wc -l` is 0.
			name: "-w0 does not wrap or terminate",
			args: []string{"-w0"}, stdin: strings.Repeat("x", 100),
			want: "eHh4" + strings.Repeat("eHh4", 32) + "eA==",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, "base64", test.args, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("base64: %v", err)
			}
			if got != test.want {
				t.Fatalf("base64(%q) = %q, want %q", test.stdin, got, test.want)
			}
		})
	}
}

// The default wrap is GNU's 76 columns, which is why a 100-byte input comes back
// as two lines.
func TestBase64_wrapsAt76(t *testing.T) {
	// When
	got, _, err := runFilter(t, "base64", nil, strings.Repeat("x", 100))

	// Then
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 || len(lines[0]) != 76 {
		t.Fatalf("base64 produced %d lines, first %d columns; want two lines wrapped at 76", len(lines), len(lines[0]))
	}
}

// Rubbish is refused rather than decoded to nearly the right bytes. A truncated
// download that comes back almost correct is worse than one that says it is
// broken.
func TestBase64_refusesInvalidInput(t *testing.T) {
	// When
	_, _, err := runFilter(t, "base64", []string{"-d"}, "not base64 at all!!\n")

	// Then
	if err == nil {
		t.Fatal("base64 -d accepted rubbish")
	}
}

// The format is `<hex>  <name>`, two spaces.
//
// The coreutils build on this machine prints `<hex> *<name>` instead, because it
// defaults to binary mode on Windows and marks it. Two spaces is chosen anyway:
// it is what GNU prints everywhere else, what a published checksum looks like,
// and what a script comparing against one will have. Nothing is lost, because
// this never translates line endings, so both modes hash the same bytes.
func TestChecksums(t *testing.T) {
	tests := []struct {
		name  string
		sum   string
		stdin string
		want  string
	}{
		{
			name: "sha256sum of hello", sum: "sha256sum", stdin: "hello",
			want: "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  -\n",
		},
		{
			name: "md5sum of hello", sum: "md5sum", stdin: "hello",
			want: "5d41402abc4b2a76b9719d911017c592  -\n",
		},
		{
			// The empty digest is worth pinning: it is the one value a broken
			// implementation is most likely to produce by accident.
			name: "sha256sum of nothing", sum: "sha256sum", stdin: "",
			want: "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  -\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// When
			got, _, err := runFilter(t, test.sum, nil, test.stdin)

			// Then
			if err != nil {
				t.Fatalf("%s: %v", test.sum, err)
			}
			if got != test.want {
				t.Fatalf("%s = %q, want %q", test.sum, got, test.want)
			}
		})
	}
}

// -c is the half that matters after a download, and it has to fail loudly.
func TestSha256sum_check(t *testing.T) {
	// Given a file and a list of sums for it
	directory := t.TempDir()
	payload := filepath.Join(directory, "payload.bin")
	if err := os.WriteFile(payload, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	good := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824  " + filepath.ToSlash(payload)
	bad := strings.Repeat("0", 64) + "  " + filepath.ToSlash(payload)

	t.Run("a matching sum is OK", func(t *testing.T) {
		// When
		got, _, err := runFilter(t, "sha256sum", []string{"-c"}, good+"\n")

		// Then
		if err != nil {
			t.Fatalf("sha256sum -c: %v", err)
		}
		if !strings.HasSuffix(strings.TrimSpace(got), ": OK") {
			t.Fatalf("output = %q, want it to end in OK", got)
		}
	})

	t.Run("a wrong sum FAILS and the status says so", func(t *testing.T) {
		// When
		got, _, err := runFilter(t, "sha256sum", []string{"-c"}, bad+"\n")

		// Then
		if err == nil {
			t.Fatal("sha256sum -c reported success for a wrong sum")
		}
		if !strings.Contains(got, ": FAILED") {
			t.Fatalf("output = %q, want FAILED on the line", got)
		}
	})

	t.Run("the asterisk form is accepted too", func(t *testing.T) {
		// A list produced by the coreutils build on this machine looks like
		// `<hex> *<name>`, so refusing that spelling would refuse the sums file
		// a Windows user most likely has.
		binary := strings.Replace(good, "  ", " *", 1)

		// When
		got, _, err := runFilter(t, "sha256sum", []string{"-c"}, binary+"\n")

		// Then
		if err != nil {
			t.Fatalf("sha256sum -c: %v, output %q", err, got)
		}
		if !strings.HasSuffix(strings.TrimSpace(got), ": OK") {
			t.Fatalf("output = %q, want it to end in OK", got)
		}
	})

	t.Run("a list of nothing but garbage does not come back clean", func(t *testing.T) {
		// GNU warns and continues; the status is what a script reads, and a file
		// of sums that parsed as nothing must not report success.
		// When
		_, _, err := runFilter(t, "sha256sum", []string{"-c"}, "this is not a checksum line\n")

		// Then
		if err == nil {
			t.Fatal("sha256sum -c reported success for a file with no valid lines")
		}
	})
}
