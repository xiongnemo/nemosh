package oilsspec_test

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xiongnemo/nemosh/internal/testutil/oilsspec"
)

// root is tests/oils, the Oils spec suite as scripts/oils-vendor.sh copies it.
var root = filepath.Join("..", "..", "..", "tests", "oils")

// The vendored copy is what upstream.json says it is, file for file: none edited, none
// missing and none added. Its cases run as upstream wrote them, so a case edited until it
// passed would be a measurement faked; a refresh goes through scripts/oils-vendor.sh,
// which rewrites the record along with the copy.
func TestVendoredCopyIsUpstreams(t *testing.T) {
	record := readUpstream(t)

	for name, file := range record.Files {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		if sum := sha256.Sum256(data); hex.EncodeToString(sum[:]) != file.SHA256 {
			t.Errorf("%s is not what Oils %.12s holds; nothing in tests/oils is edited by hand", name, record.Commit)
		}
	}
	err := filepath.WalkDir(filepath.Join(root, "spec"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		name, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if _, listed := record.Files[filepath.ToSlash(name)]; !listed {
			t.Errorf("%s is not in upstream.json; tests/oils/spec holds only what scripts/oils-vendor.sh copies", filepath.ToSlash(name))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A file Oils keeps executable is recorded executable here too, since cases run
// spec/testdata/echo.sh and its like by path, and one Oils does not keep executable is not. A
// Windows checkout has no executable bit, so it is git's record of the mode that is asked.
func TestVendoredCopyKeepsUpstreamsModes(t *testing.T) {
	record := readUpstream(t)
	listing, err := exec.Command("git", "-C", root, "ls-files", "-s", "--", ".").Output()
	if err != nil {
		t.Skipf("no git to ask for the modes: %v", err)
	}
	modes := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(listing)), "\n") {
		if fields, name, found := strings.Cut(line, "\t"); found {
			modes[name] = strings.Fields(fields)[0]
		}
	}

	for name, file := range record.Files {
		if mode := modes[name]; mode != file.Mode {
			t.Errorf("%s has mode %q in git, and Oils records it as %s", name, mode, file.Mode)
		}
	}
}

// THIRD-PARTY-NOTICES.md and tests/oils/README.md name the commit the copy came from and
// its date, so a refresh that moves the copy cannot leave its attribution behind.
func TestNoticesNameTheVendoredCommit(t *testing.T) {
	record := readUpstream(t)
	commit := "commit `" + record.Commit[:7] + "`"

	notices, err := os.ReadFile(filepath.Join(root, "..", "..", "THIRD-PARTY-NOTICES.md"))
	if err != nil {
		t.Fatal(err)
	}
	_, entry, found := strings.Cut(string(notices), "\n## Andy Chu and the Oils contributors")
	if !found {
		t.Fatal("THIRD-PARTY-NOTICES.md has no entry for Oils")
	}
	entry, _, _ = strings.Cut(entry, "\n---\n")
	readme, err := os.ReadFile(filepath.Join(root, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	for file, text := range map[string]string{"THIRD-PARTY-NOTICES.md": entry, "tests/oils/README.md": string(readme)} {
		for _, want := range []string{commit, record.Committed} {
			if !strings.Contains(text, want) {
				t.Errorf("%s does not say %q, which upstream.json records", file, want)
			}
		}
	}
}

func readUpstream(t *testing.T) oilsspec.Upstream {
	t.Helper()
	record, err := oilsspec.ReadUpstream(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(record.Commit) != 40 || len(record.Files) == 0 {
		t.Fatalf("upstream.json records commit %q and %d files", record.Commit, len(record.Files))
	}
	return record
}
