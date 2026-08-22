package applets_test

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Link entries, end to end. The unit tests in archive_containment_test.go cover
// safeLinkTarget's own branches; these prove the archivers actually *call* it, and
// that a refused link leaves nothing behind.
//
// This is the hazard the containment list calls "a symlink or hardlink entry whose
// target leaves the extraction root". A link is harmless in itself -- what escapes
// is a *later* entry written through it -- so the test asserts both halves: the
// link is not created, and the sentinel outside the root is untouched.

// buildTarWithLinks writes an archive holding link entries, which archive/tar will
// do without complaint. That is the point: a hostile archive is built by something
// that does not share our rules.
func buildTarWithLinks(t *testing.T, links []tarLink, files map[string]string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for _, link := range links {
		header := &tar.Header{Name: link.name, Linkname: link.target, Typeflag: link.kind, Mode: 0o777}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range files {
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

type tarLink struct {
	name, target string
	kind         byte
}

// Every escaping link target, through tar, with a sentinel outside the root.
func TestTar_refusesALinkThatLeavesTheRoot(t *testing.T) {
	for _, hazard := range []struct {
		reason string
		link   tarLink
	}{
		{reason: "a symlink one level up", link: tarLink{name: "a", target: "../escape", kind: tar.TypeSymlink}},
		{reason: "a symlink to the parent itself", link: tarLink{name: "a", target: "..", kind: tar.TypeSymlink}},
		{reason: "a symlink escaping from a subdirectory", link: tarLink{name: "sub/a", target: "../../escape", kind: tar.TypeSymlink}},
		{reason: "a symlink escaping after cleaning", link: tarLink{name: "sub/a", target: "x/../../../escape", kind: tar.TypeSymlink}},
		{reason: "an absolute symlink", link: tarLink{name: "a", target: "/etc/passwd", kind: tar.TypeSymlink}},
		{reason: "a drive-qualified symlink", link: tarLink{name: "a", target: `C:\windows\system32`, kind: tar.TypeSymlink}},
		{reason: "a backslash symlink escape", link: tarLink{name: "a", target: `..\escape`, kind: tar.TypeSymlink}},
		{reason: "an empty symlink target", link: tarLink{name: "a", target: "", kind: tar.TypeSymlink}},
		// A hardlink names an existing file rather than a path to follow, but the
		// escape is the same shape and gets the same check.
		{reason: "a hardlink one level up", link: tarLink{name: "a", target: "../escape", kind: tar.TypeLink}},
		{reason: "an absolute hardlink", link: tarLink{name: "a", target: "/etc/passwd", kind: tar.TypeLink}},
	} {
		t.Run(hazard.reason, func(t *testing.T) {
			outer := t.TempDir()
			root := filepath.Join(outer, "root")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			// A file outside the root that a followed link would reach.
			if err := os.WriteFile(filepath.Join(outer, "escape"), []byte("secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			archive := buildTarWithLinks(t, []tarLink{hazard.link}, map[string]string{"honest.txt": "kept"})
			if err := os.WriteFile(filepath.Join(root, "a.tar"), archive, 0o600); err != nil {
				t.Fatal(err)
			}

			_, stderr, err := runSmall(t, root, "", "tar", "-x", "-f", "a.tar")
			if err != nil {
				t.Fatalf("tar -x: %v (%s)", err, stderr)
			}
			if !strings.Contains(stderr, "skipping") {
				t.Fatalf("tar accepted the link without complaint (stderr %q)", stderr)
			}
			// The link itself was not created under any name.
			if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(hazard.link.name))); err == nil {
				t.Fatalf("the refused link %q was created anyway", hazard.link.name)
			}
			// The honest entry alongside it still arrives.
			if _, err := os.Stat(filepath.Join(root, "honest.txt")); err != nil {
				t.Fatalf("the honest entry was lost with the link: %v", err)
			}
			// And the sentinel is exactly as it was: not overwritten, not removed.
			content, err := os.ReadFile(filepath.Join(outer, "escape"))
			if err != nil || string(content) != "secret" {
				t.Fatalf("the file outside the root read %q (%v)", content, err)
			}
		})
	}
}

// A link whose target *is* inside the root is not refused for being a link. It may
// still not be creatable -- Windows needs a privilege for a symlink -- but the
// containment check must not be what stops it, or the check is refusing the wrong
// thing and the reason in the message would be wrong.
func TestTar_doesNotRefuseAContainedLinkForItsTarget(t *testing.T) {
	root := t.TempDir()
	archive := buildTarWithLinks(t,
		[]tarLink{{name: "sub/a", target: "../b.txt", kind: tar.TypeSymlink}},
		map[string]string{"b.txt": "target"})
	if err := os.WriteFile(filepath.Join(root, "a.tar"), archive, 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runSmall(t, root, "", "tar", "-x", "-f", "a.tar")
	if err != nil {
		t.Fatalf("tar -x: %v (%s)", err, stderr)
	}
	// Whatever else it says, it must not say the link leaves the archive.
	if strings.Contains(stderr, "links outside") {
		t.Fatalf("a contained link was refused as an escape: %q", stderr)
	}
}

// The same, through cpio, whose symlinks are a mode bit plus the target as the
// entry's *data* -- a different shape reaching the same helper.
func TestCpio_refusesALinkThatLeavesTheRoot(t *testing.T) {
	const symlinkMode = 0o120777
	for _, hazard := range []struct{ reason, name, target string }{
		{reason: "one level up", name: "a", target: "../escape"},
		{reason: "from a subdirectory", name: "sub/a", target: "../../escape"},
		{reason: "absolute", name: "a", target: "/etc/passwd"},
		{reason: "drive-qualified", name: "a", target: `C:\windows`},
		{reason: "backslash escape", name: "a", target: `..\escape`},
	} {
		t.Run(hazard.reason, func(t *testing.T) {
			outer := t.TempDir()
			root := filepath.Join(outer, "root")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(outer, "escape"), []byte("secret"), 0o600); err != nil {
				t.Fatal(err)
			}
			archive := buildCpio(t, []cpioTestEntry{
				// The target is the data, and the mode says symlink.
				{name: hazard.name, content: hazard.target, mode: symlinkMode},
				{name: "honest.txt", content: "kept"},
			})
			if err := os.WriteFile(filepath.Join(root, "a.cpio"), archive, 0o600); err != nil {
				t.Fatal(err)
			}

			_, stderr, err := runSmall(t, root, "", "cpio", "-i", "-d", "-F", "a.cpio")
			if err != nil {
				t.Fatalf("cpio -i: %v (%s)", err, stderr)
			}
			if !strings.Contains(stderr, "skipping") {
				t.Fatalf("cpio accepted the link without complaint (stderr %q)", stderr)
			}
			if _, err := os.Lstat(filepath.Join(root, filepath.FromSlash(hazard.name))); err == nil {
				t.Fatalf("the refused link %q was created anyway", hazard.name)
			}
			// The honest entry *after* it still arrives, which is the harder half: a
			// refusal that lost its place in the stream would drop this too, and a
			// symlink entry's data has to be consumed for the stream to stay aligned.
			content, err := os.ReadFile(filepath.Join(root, "honest.txt"))
			if err != nil || string(content) != "kept" {
				t.Fatalf("the honest entry read %q (%v): the stream lost alignment", content, err)
			}
			if content, err := os.ReadFile(filepath.Join(outer, "escape")); err != nil || string(content) != "secret" {
				t.Fatalf("the file outside the root read %q (%v)", content, err)
			}
		})
	}
}

// A contained cpio symlink is written as a regular file holding its target, and
// says so. Windows needs a privilege to create a real one, and the choice between
// dropping the content and following it silently is worth being explicit about.
func TestCpio_writesAContainedSymlinkAsItsTarget(t *testing.T) {
	root := t.TempDir()
	archive := buildCpio(t, []cpioTestEntry{
		{name: "link", content: "b.txt", mode: 0o120777},
		{name: "b.txt", content: "target\n"},
	})
	if err := os.WriteFile(filepath.Join(root, "a.cpio"), archive, 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runSmall(t, root, "", "cpio", "-i", "-d", "-F", "a.cpio")
	if err != nil {
		t.Fatalf("cpio -i: %v (%s)", err, stderr)
	}
	// It must say what it did, because a file where a link was expected is a
	// surprise worth one line of output.
	if !strings.Contains(stderr, "symlink") {
		t.Fatalf("cpio did not report writing the symlink as a file: %q", stderr)
	}
	got, err := os.ReadFile(filepath.Join(root, "link"))
	if err != nil {
		t.Fatalf("the symlink entry produced nothing: %v", err)
	}
	if string(got) != "b.txt" {
		t.Fatalf("the file holds %q, want the link target", got)
	}
	// And the entry after it is intact, so the target's bytes were consumed exactly.
	if content, err := os.ReadFile(filepath.Join(root, "b.txt")); err != nil || string(content) != "target\n" {
		t.Fatalf("the following entry read %q (%v)", content, err)
	}
}

// A cpio entry that is neither file, directory nor symlink -- a device or socket
// node -- is skipped with a reason rather than turned into an empty file, which
// would misrepresent what the archive holds.
func TestCpio_skipsAnEntryItCannotCreate(t *testing.T) {
	root := t.TempDir()
	const characterDeviceMode = 0o020666
	archive := buildCpio(t, []cpioTestEntry{
		{name: "dev-node", content: "", mode: characterDeviceMode},
		{name: "honest.txt", content: "kept"},
	})
	if err := os.WriteFile(filepath.Join(root, "a.cpio"), archive, 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runSmall(t, root, "", "cpio", "-i", "-d", "-F", "a.cpio")
	if err != nil {
		t.Fatalf("cpio -i: %v (%s)", err, stderr)
	}
	if !strings.Contains(stderr, "not a file, directory or symlink") {
		t.Fatalf("cpio did not say why it skipped the node: %q", stderr)
	}
	if _, err := os.Stat(filepath.Join(root, "dev-node")); err == nil {
		t.Fatal("cpio created something for a device node")
	}
	if content, err := os.ReadFile(filepath.Join(root, "honest.txt")); err != nil || string(content) != "kept" {
		t.Fatalf("the honest entry read %q (%v)", content, err)
	}
}

// A directory entry is created, and one whose name is hostile is not. cpio carries
// directories as a mode bit with no data, so this is the third entry shape.
func TestCpio_createsDirectoriesAndRefusesHostileOnes(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "root")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	const directoryMode = 0o040755
	archive := buildCpio(t, []cpioTestEntry{
		{name: "sub", mode: directoryMode},
		{name: "../escape-dir", mode: directoryMode},
		{name: "sub/inner.txt", content: "in a directory\n"},
	})
	if err := os.WriteFile(filepath.Join(root, "a.cpio"), archive, 0o600); err != nil {
		t.Fatal(err)
	}

	_, stderr, err := runSmall(t, root, "", "cpio", "-i", "-F", "a.cpio")
	if err != nil {
		t.Fatalf("cpio -i: %v (%s)", err, stderr)
	}
	info, err := os.Stat(filepath.Join(root, "sub"))
	if err != nil || !info.IsDir() {
		t.Fatalf("the directory entry was not created: %v", err)
	}
	// The file inside it arrives without -d, because its parent came first.
	if content, err := os.ReadFile(filepath.Join(root, "sub", "inner.txt")); err != nil || string(content) != "in a directory\n" {
		t.Fatalf("the file inside the directory read %q (%v)", content, err)
	}
	if _, err := os.Stat(filepath.Join(outer, "escape-dir")); err == nil {
		t.Fatal("cpio created a directory outside the root")
	}
}
