package applets_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Where an archive entry is allowed to land.
//
// An archive is untrusted input that names its own destinations, so this is the
// most security-relevant code in the applet set. Every hazard below gets a
// hand-built archive, and every test asserts two things: the entry was refused,
// **and nothing was written outside the extraction root**. The second is the one
// that matters -- a refusal reported after the file was created is no refusal.

// buildTar makes an archive with exactly the entries given, bypassing any name
// checking a writer might do. That is the point: a hostile archive is built by
// something that does not care about our rules.
func buildTar(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for name, content := range entries {
		header := &tar.Header{Name: name, Mode: 0o644, Size: int64(len(content)), Typeflag: tar.TypeReg}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func buildZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for name, content := range entries {
		file, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// The hazard list. Each name is one an archive can legally contain and that must
// not be created.
var hostileArchiveNames = []struct {
	name   string
	reason string
}{
	{name: "../escape.txt", reason: "escapes the root"},
	{name: "a/b/../../../escape.txt", reason: "escapes after cleaning, which a prefix check would miss"},
	{name: "/absolute.txt", reason: "absolute"},
	{name: `C:\windows\evil.txt`, reason: "drive-qualified with backslashes"},
	{name: "C:/windows/evil.txt", reason: "drive-qualified with slashes"},
	{name: "C:relative.txt", reason: "drive-relative, resolved against that drive's own directory"},
	// Backslashes are normalised to separators rather than refused, so this is
	// caught by the escape check after normalisation -- which is the point of
	// normalising rather than refusing.
	{name: `a\..\..\escape.txt`, reason: "escapes once its backslashes are normalised"},
	// Windows resolves these in *every* directory, so extracting one writes to
	// the device and silently loses the data.
	{name: "NUL", reason: "reserved device name"},
	{name: "nul.txt", reason: "reserved device name with an extension"},
	{name: "sub/CON", reason: "reserved device name in a subdirectory"},
	{name: "COM1.tar.gz", reason: "reserved device name with two extensions"},
	// Windows strips a trailing dot or space, so `evil.` and `evil` collide.
	{name: "evil.", reason: "trailing dot, which Windows strips"},
	{name: "evil ", reason: "trailing space, which Windows strips"},
	{name: "..", reason: "the parent itself"},
}

func TestTar_refusesEveryHostileEntryAndWritesNothingOutside(t *testing.T) {
	for _, hazard := range hostileArchiveNames {
		t.Run(hazard.reason, func(t *testing.T) {
			// A sentinel one level above the extraction root, so an escape is
			// visible as a created file rather than only as a missing refusal.
			outer := t.TempDir()
			root := filepath.Join(outer, "root")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := buildTar(t, map[string]string{hazard.name: "payload", "honest.txt": "kept"})
			if err := os.WriteFile(filepath.Join(root, "a.tar"), archive, 0o600); err != nil {
				t.Fatal(err)
			}

			_, stderr, err := runSmall(t, root, "", "tar", "-x", "-f", "a.tar")
			if err != nil {
				t.Fatalf("tar -x: %v (%s)", err, stderr)
			}
			if !strings.Contains(stderr, "skipping") {
				t.Fatalf("tar extracted %q without complaint (stderr %q)", hazard.name, stderr)
			}
			// The honest entry alongside it still arrives: one hostile name must
			// not cost the rest of the archive.
			if _, err := os.Stat(filepath.Join(root, "honest.txt")); err != nil {
				t.Fatalf("the honest entry was lost along with the hostile one")
			}
			assertNothingOutside(t, outer, root)
		})
	}
}

func TestUnzip_refusesEveryHostileEntryAndWritesNothingOutside(t *testing.T) {
	for _, hazard := range hostileArchiveNames {
		t.Run(hazard.reason, func(t *testing.T) {
			outer := t.TempDir()
			root := filepath.Join(outer, "root")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := buildZip(t, map[string]string{hazard.name: "payload", "honest.txt": "kept"})
			if err := os.WriteFile(filepath.Join(root, "a.zip"), archive, 0o600); err != nil {
				t.Fatal(err)
			}
			_, stderr, err := runSmall(t, root, "", "unzip", "a.zip")
			if err != nil {
				t.Fatalf("unzip: %v (%s)", err, stderr)
			}
			if !strings.Contains(stderr, "skipping") {
				t.Fatalf("unzip extracted %q without complaint (stderr %q)", hazard.name, stderr)
			}
			if _, err := os.Stat(filepath.Join(root, "honest.txt")); err != nil {
				t.Fatal("the honest entry was lost along with the hostile one")
			}
			assertNothingOutside(t, outer, root)
		})
	}
}

// assertNothingOutside walks the directory *containing* the extraction root and
// fails if anything appeared beside it. This is the assertion that actually
// catches an escape; checking only for a diagnostic would pass a version that
// complained and then wrote the file anyway.
func assertNothingOutside(t *testing.T, outer, root string) {
	t.Helper()
	entries, err := os.ReadDir(outer)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if filepath.Join(outer, entry.Name()) != root {
			t.Fatalf("something was written outside the extraction root: %s", entry.Name())
		}
	}
}

// Two entries differing only in case are one file on NTFS, so the second would
// silently overwrite the first -- a way to smuggle content past a review that
// read the listing.
func TestArchive_refusesACaseInsensitiveCollision(t *testing.T) {
	root := t.TempDir()
	archive := buildTar(t, map[string]string{"Foo.txt": "first", "foo.txt": "second"})
	if err := os.WriteFile(filepath.Join(root, "a.tar"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runSmall(t, root, "", "tar", "-x", "-f", "a.tar")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "collides") {
		t.Fatalf("tar extracted a case-insensitive collision without complaint: %q", stderr)
	}
}

// -j flattens the stored directories, which changes the name *after* the first
// check -- so it has to be checked again. Flattening `sub/NUL` to `NUL` would
// otherwise slip a device name past.
func TestUnzip_rechecksAfterFlattening(t *testing.T) {
	root := t.TempDir()
	archive := buildZip(t, map[string]string{"sub/NUL": "payload"})
	if err := os.WriteFile(filepath.Join(root, "a.zip"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runSmall(t, root, "", "unzip", "-j", "a.zip")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "skipping") {
		t.Fatalf("unzip -j flattened a name into a device name: %q", stderr)
	}
}

// Listing does *not* check, and that is deliberate: listing is how somebody
// inspects an archive they do not trust, so hiding the hostile entry would defeat
// the purpose.
func TestTar_listingShowsHostileNames(t *testing.T) {
	root := t.TempDir()
	archive := buildTar(t, map[string]string{"../escape.txt": "payload"})
	if err := os.WriteFile(filepath.Join(root, "a.tar"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	stdout, _, err := runSmall(t, root, "", "tar", "-t", "-f", "a.tar")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout, "../escape.txt") {
		t.Fatalf("tar -t hid the hostile entry: %q", stdout)
	}
}

// An honest archive must round-trip, or the containment is worthless for being
// too strict.
func TestTar_roundTripsAnHonestArchive(t *testing.T) {
	dir := writeSmallFixture(t, map[string]string{
		"src/a.txt":     "first\n",
		"src/sub/b.txt": "second\n",
	})
	if _, stderr, err := runSmall(t, dir, "", "tar", "-c", "-f", "out.tar", "src"); err != nil {
		t.Fatalf("tar -c: %v (%s)", err, stderr)
	}
	listing, _, err := runSmall(t, dir, "", "tar", "-t", "-f", "out.tar")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"src/a.txt", "src/sub/b.txt"} {
		if !strings.Contains(listing, want) {
			t.Fatalf("tar -t = %q, missing %q", listing, want)
		}
	}
	// Stored with forward slashes, which is what makes the archive readable by
	// tar on any platform -- and what stops this build writing the very
	// drive-qualified names its own extractor refuses.
	if strings.Contains(listing, `\`) {
		t.Fatalf("tar stored a backslash in an entry name: %q", listing)
	}

	// Extract into a fresh directory and compare.
	target := filepath.Join(dir, "out")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, stderr, err := runSmall(t, dir, "", "tar", "-x", "-f", "out.tar", "-C", "out"); err != nil {
		t.Fatalf("tar -x: %v (%s)", err, stderr)
	}
	for name, want := range map[string]string{
		filepath.Join(target, "src", "a.txt"):        "first\n",
		filepath.Join(target, "src", "sub", "b.txt"): "second\n",
	} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("extraction lost %s: %v", name, err)
		}
		if string(data) != want {
			t.Fatalf("%s = %q, want %q", name, data, want)
		}
	}
	// And through gzip, which is what `tar -czf` has to do without a second
	// program.
	if _, stderr, err := runSmall(t, dir, "", "tar", "-c", "-z", "-f", "out.tgz", "src"); err != nil {
		t.Fatalf("tar -cz: %v (%s)", err, stderr)
	}
	gzipped, _, err := runSmall(t, dir, "", "tar", "-t", "-z", "-f", "out.tgz")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(gzipped, "src/a.txt") {
		t.Fatalf("tar -tzf = %q", gzipped)
	}
}

func TestUnzip_listsTestsAndExtracts(t *testing.T) {
	root := t.TempDir()
	archive := buildZip(t, map[string]string{"a.txt": "first\n", "sub/b.txt": "second\n"})
	if err := os.WriteFile(filepath.Join(root, "a.zip"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	listing, _, err := runSmall(t, root, "", "unzip", "-l", "a.zip")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Length", "a.txt", "sub/b.txt", "2 files"} {
		if !strings.Contains(listing, want) {
			t.Fatalf("unzip -l = %q, missing %q", listing, want)
		}
	}
	if _, _, err := runSmall(t, root, "", "unzip", "-t", "a.zip"); err != nil {
		t.Fatalf("unzip -t on a good archive: %v", err)
	}
	// -p writes a member to stdout without touching the disk.
	stdout, _, err := runSmall(t, root, "", "unzip", "-p", "a.zip", "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stdout != "first\n" {
		t.Fatalf("unzip -p = %q", stdout)
	}
	if _, _, err := runSmall(t, root, "", "unzip", "a.zip"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "sub", "b.txt"))
	if err != nil || string(data) != "second\n" {
		t.Fatalf("unzip did not extract sub/b.txt: %v %q", err, data)
	}
	// A second run without -o or -n leaves the file alone and says so, because
	// there is no prompt to ask at.
	_, stderr, err := runSmall(t, root, "", "unzip", "a.zip")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr, "exists") {
		t.Fatalf("unzip overwrote silently: %q", stderr)
	}
	// zip cannot be read from a pipe, and saying so beats pretending.
	if _, _, err := runSmall(t, root, string(archive), "unzip"); err == nil {
		t.Fatal("unzip accepted a piped archive")
	}
}

// PowerShell's Compress-Archive writes backslash-separated entry names --
// `src\a.txt` -- against the zip specification, which mandates `/`. Refusing them
// made every PowerShell-made zip completely unextractable, and PowerShell is the
// most likely producer of a zip on this platform.
//
// Normalising instead is not a weakening: the hazard table above includes
// `a\..\..\escape.txt`, which is still caught, because after normalisation the
// escape check does the work.
func TestUnzip_extractsPowerShellBackslashNames(t *testing.T) {
	root := t.TempDir()
	archive := buildZip(t, map[string]string{`src\a.txt`: "first\n", `src\sub\b.txt`: "second\n"})
	if err := os.WriteFile(filepath.Join(root, "ps.zip"), archive, 0o600); err != nil {
		t.Fatal(err)
	}
	_, stderr, err := runSmall(t, root, "", "unzip", "ps.zip")
	if err != nil {
		t.Fatalf("unzip: %v (%s)", err, stderr)
	}
	if strings.Contains(stderr, "skipping") {
		t.Fatalf("unzip refused a PowerShell-made archive: %q", stderr)
	}
	for name, want := range map[string]string{
		filepath.Join(root, "src", "a.txt"):        "first\n",
		filepath.Join(root, "src", "sub", "b.txt"): "second\n",
	} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("did not extract %s: %v", name, err)
		}
		if string(data) != want {
			t.Fatalf("%s = %q, want %q", name, data, want)
		}
	}
}
