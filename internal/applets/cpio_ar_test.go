package applets_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// cpio and ar, the two archivers with no library behind them: the headers are
// built here byte by byte, in both directions, so the tests are the only thing
// standing between a field written into the wrong column and an archive that
// looks fine until something else reads it.
//
// Both go through the same containment helper tar and unzip use, and both are run
// against the whole hostile-name table for exactly that reason -- a shared helper
// is only shared if every caller reaches it.

// buildCpio writes a newc archive with the entries given, in order, bypassing
// every check. A hostile archive is built by something that does not care about
// our rules, which is why this does not use the applet's own writer.
func buildCpio(t *testing.T, entries []cpioTestEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	for index, entry := range entries {
		mode := entry.mode
		if mode == 0 {
			mode = 0o100644
		}
		name := entry.name + "\x00"
		fields := []int64{
			int64(index + 1), int64(mode), 0, 0, 1, 1700000000,
			int64(len(entry.content)), 0, 0, 0, 0, int64(len(name)), 0,
		}
		buffer.WriteString("070701")
		for _, value := range fields {
			fmt.Fprintf(&buffer, "%08X", uint32(value))
		}
		buffer.WriteString(name)
		writeTestPadding(&buffer, 110+len(name))
		buffer.WriteString(entry.content)
		writeTestPadding(&buffer, len(entry.content))
	}
	// The trailer, without which a reader is right to call the archive truncated.
	trailer := "TRAILER!!!\x00"
	buffer.WriteString("070701")
	for index := 0; index < 13; index++ {
		value := 0
		if index == 11 {
			value = len(trailer)
		}
		if index == 4 {
			value = 1
		}
		fmt.Fprintf(&buffer, "%08X", uint32(value))
	}
	buffer.WriteString(trailer)
	writeTestPadding(&buffer, 110+len(trailer))
	return buffer.Bytes()
}

type cpioTestEntry struct {
	name    string
	content string
	mode    int
}

func writeTestPadding(buffer *bytes.Buffer, length int) {
	for length%4 != 0 {
		buffer.WriteByte(0)
		length++
	}
}

// buildAr writes an ar archive, again without any name checking.
func buildAr(t *testing.T, entries []cpioTestEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	buffer.WriteString("!<arch>\n")
	for _, entry := range entries {
		// The stored name keeps GNU's `/` terminator. A name too long for the
		// sixteen columns would need the long-name table, which these fixtures
		// avoid by staying short.
		header := fmt.Sprintf("%-16s%-12d%-6d%-6d%-8o%-10d`\n",
			entry.name+"/", 1700000000, 0, 0, 0o644, len(entry.content))
		if len(header) != 60 {
			t.Fatalf("built a %d-byte ar header for %q", len(header), entry.name)
		}
		buffer.WriteString(header)
		buffer.WriteString(entry.content)
		if len(entry.content)%2 == 1 {
			buffer.WriteString("\n")
		}
	}
	return buffer.Bytes()
}

func TestCpio_refusesEveryHostileEntryAndWritesNothingOutside(t *testing.T) {
	for _, hazard := range hostileArchiveNames {
		t.Run(hazard.reason, func(t *testing.T) {
			outer := t.TempDir()
			root := filepath.Join(outer, "root")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := buildCpio(t, []cpioTestEntry{
				{name: hazard.name, content: "payload"},
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
				t.Fatalf("cpio extracted %q without complaint (stderr %q)", hazard.name, stderr)
			}
			// The honest entry after it still arrives, which is the harder half:
			// a refusal that lost its place in the stream would drop this too.
			content, err := os.ReadFile(filepath.Join(root, "honest.txt"))
			if err != nil {
				t.Fatalf("the honest entry was lost along with the hostile one: %v", err)
			}
			if string(content) != "kept" {
				t.Fatalf("the honest entry read %q, so the stream lost its alignment", content)
			}
			assertNothingOutside(t, outer, root)
		})
	}
}

func TestAr_refusesEveryHostileEntryAndWritesNothingOutside(t *testing.T) {
	for _, hazard := range hostileArchiveNames {
		// ar stores a flat name in sixteen columns, so the two long fixtures
		// cannot be expressed in one -- and a truncated name is a different test
		// from the one intended.
		if len(hazard.name)+1 > 16 {
			continue
		}
		t.Run(hazard.reason, func(t *testing.T) {
			outer := t.TempDir()
			root := filepath.Join(outer, "root")
			if err := os.Mkdir(root, 0o755); err != nil {
				t.Fatal(err)
			}
			archive := buildAr(t, []cpioTestEntry{
				{name: hazard.name, content: "payload"},
				{name: "honest.txt", content: "kept"},
			})
			if err := os.WriteFile(filepath.Join(root, "a.a"), archive, 0o600); err != nil {
				t.Fatal(err)
			}

			_, stderr, err := runSmall(t, root, "", "ar", "x", "a.a")
			if err != nil {
				t.Fatalf("ar x: %v (%s)", err, stderr)
			}
			if !strings.Contains(stderr, "skipping") {
				t.Fatalf("ar extracted %q without complaint (stderr %q)", hazard.name, stderr)
			}
			if content, err := os.ReadFile(filepath.Join(root, "honest.txt")); err != nil || string(content) != "kept" {
				t.Fatalf("the honest member was lost or misread: %q %v", content, err)
			}
			assertNothingOutside(t, outer, root)
		})
	}
}
