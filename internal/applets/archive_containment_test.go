package applets

import (
	"path"
	"strings"
	"testing"
)

// The containment helper, tested directly.
//
// `archive_path_test.go` drives it end to end through tar and unzip, which is the
// test that matters -- a refusal reported after the file was created is no refusal.
// But end-to-end cannot reach every branch: several are defensive, and one whole
// function, `safeLinkTarget`, had **no coverage at all** despite being called from
// tar.go and cpio_io.go. A link is the second way out of an extraction root, so
// that gap was in the most security-relevant function of the set.
//
// In package `applets` rather than `applets_test` because both helpers are
// unexported, and exporting them for a test would be the wrong trade.

func TestSafeArchivePath_acceptsAnHonestName(t *testing.T) {
	for _, test := range []struct{ name, want string }{
		{name: "a.txt", want: "a.txt"},
		{name: "sub/a.txt", want: "sub/a.txt"},
		{name: "a/b/c/d.txt", want: "a/b/c/d.txt"},
		// Cleaning happens, so a redundant name normalises rather than being refused.
		{name: "./a.txt", want: "a.txt"},
		{name: "a//b.txt", want: "a/b.txt"},
		{name: "a/./b.txt", want: "a/b.txt"},
		{name: "a/x/../b.txt", want: "a/b.txt"},
		// A trailing slash is how tar names a directory.
		{name: "sub/", want: "sub"},
		// Backslashes normalise to separators rather than being refused, which is
		// the PowerShell interoperability decision: Compress-Archive writes them.
		{name: `sub\a.txt`, want: "sub/a.txt"},
		{name: `a\b\c.txt`, want: "a/b/c.txt"},
		// A dot inside a component is a file extension, not a trailing dot.
		{name: "a.tar.gz", want: "a.tar.gz"},
		// Not a reserved name: the stem has to match exactly.
		{name: "nullify.txt", want: "nullify.txt"},
		{name: "console.log", want: "console.log"},
		{name: "com10.txt", want: "com10.txt"},
		{name: "lpt0.txt", want: "lpt0.txt"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := safeArchivePath(test.name)
			if err != nil {
				t.Fatalf("safeArchivePath(%q) refused an honest name: %v", test.name, err)
			}
			if got != test.want {
				t.Fatalf("safeArchivePath(%q) = %q, want %q", test.name, got, test.want)
			}
		})
	}
}

// Every refusal, including the branches an archive cannot easily express and which
// therefore have no end-to-end test.
func TestSafeArchivePath_refusalsAndTheirReasons(t *testing.T) {
	for _, test := range []struct {
		name string
		// A fragment the message must contain, so a refusal cannot pass this test
		// by being refused for some unrelated reason.
		because string
	}{
		{name: "", because: "empty"},
		{name: "..", because: "escapes"},
		{name: "../x", because: "escapes"},
		{name: "a/../../x", because: "escapes"},
		{name: `a\..\..\x`, because: "escapes"},
		{name: "/x", because: "absolute"},
		{name: "//x", because: "absolute"},
		{name: `\x`, because: "absolute"},
		{name: "C:/x", because: "drive"},
		{name: `C:\x`, because: "drive"},
		{name: "C:x", because: "drive"},
		{name: "c:x", because: "drive"},
		// A UNC path is two leading separators, which normalise to absolute.
		{name: `\\server\share\x`, because: "absolute"},
		// Resolving to the root itself, which an archive can hold and which would
		// otherwise be written *as* the extraction directory.
		{name: ".", because: "root"},
		{name: "./", because: "root"},
		{name: "a/..", because: "root"},
		{name: `.\`, because: "root"},
		// Windows strips these, so two entries collide.
		{name: "x.", because: "dot or space"},
		{name: "x ", because: "dot or space"},
		{name: "sub/x.", because: "dot or space"},
		{name: "x./y", because: "dot or space"},
		// Reserved device names, with and without extensions, at any depth.
		{name: "NUL", because: "reserved"},
		{name: "nul", because: "reserved"},
		{name: "NuL.txt", because: "reserved"},
		{name: "a/b/CON", because: "reserved"},
		{name: "PRN.tar.gz", because: "reserved"},
		{name: "AUX", because: "reserved"},
		{name: "COM1", because: "reserved"},
		{name: "com9.dat", because: "reserved"},
		{name: "LPT1", because: "reserved"},
		{name: "lpt9", because: "reserved"},
		// A reserved name as a *directory* component is just as dangerous: the
		// path cannot be created, and on some paths it resolves to the device.
		{name: "NUL/x.txt", because: "reserved"},
	} {
		label := test.name
		if label == "" {
			label = "the empty name"
		}
		t.Run(label, func(t *testing.T) {
			got, err := safeArchivePath(test.name)
			if err == nil {
				t.Fatalf("safeArchivePath(%q) = %q, want a refusal", test.name, got)
			}
			if !strings.Contains(err.Error(), test.because) {
				t.Fatalf("safeArchivePath(%q) refused with %q, which does not mention %q",
					test.name, err, test.because)
			}
			// A refusal must answer the empty string, so a caller that ignores the
			// error cannot be handed a usable path anyway.
			if got != "" {
				t.Fatalf("a refused name still answered %q", got)
			}
		})
	}
}

// checkArchiveElement's empty-and-relative branch, which safeArchivePath cannot
// reach: path.Clean removes every empty, `.` and `..` component before the loop
// runs. It is defensive, and a test is how it stays correct while unreachable --
// a future caller that skips the cleaning would depend on it.
func TestCheckArchiveElement_refusesWhatCleaningWouldHaveRemoved(t *testing.T) {
	for _, element := range []string{"", ".", ".."} {
		if err := checkArchiveElement(element, "whole/name"); err == nil {
			t.Errorf("checkArchiveElement(%q) was accepted", element)
		}
	}
	// And the whole name is quoted in the message, not just the bad component,
	// because the component alone does not tell the reader which entry it was.
	err := checkArchiveElement("NUL", "sub/NUL")
	if err == nil || !strings.Contains(err.Error(), "sub/NUL") {
		t.Fatalf("the message %v does not name the whole entry", err)
	}
}

// safeLinkTarget: the second way out of an extraction root, and the function that
// had no test at all.
//
// An entry `a -> ../../etc/passwd` is harmless in itself. What escapes is a *later*
// entry written through `a`, so the target is held to the same rule as a name --
// and resolved against the link's own directory, because that is where a relative
// target is followed from.
func TestSafeLinkTarget_acceptsATargetInsideTheRoot(t *testing.T) {
	for _, test := range []struct{ entry, target string }{
		{entry: "a", target: "b"},
		{entry: "sub/a", target: "b"},
		// Up one from `sub/` is the root, which is still inside.
		{entry: "sub/a", target: "../b"},
		{entry: "sub/deep/a", target: "../../b"},
		{entry: "sub/a", target: "./b"},
		{entry: "a", target: "sub/b"},
		// Resolves to the root itself, which is inside it.
		{entry: "sub/a", target: ".."},
		// Backslashes normalise, for the same reason a name's do.
		{entry: "sub/a", target: `..\b`},
	} {
		t.Run(test.entry+" -> "+test.target, func(t *testing.T) {
			if err := safeLinkTarget(test.entry, test.target); err != nil {
				t.Fatalf("safeLinkTarget(%q, %q) refused a contained target: %v",
					test.entry, test.target, err)
			}
		})
	}
}

func TestSafeLinkTarget_refusesEveryEscape(t *testing.T) {
	for _, test := range []struct{ entry, target, because string }{
		{entry: "a", target: "", because: "empty"},
		// One level up from the root.
		{entry: "a", target: "../escape", because: "outside"},
		{entry: "a", target: "..", because: "outside"},
		{entry: "sub/a", target: "../../escape", because: "outside"},
		{entry: "sub/a", target: "../..", because: "outside"},
		// Deep enough to pass a naive prefix check and still escape.
		{entry: "sub/a", target: "x/../../../escape", because: "outside"},
		// Absolute, refused outright rather than resolved.
		{entry: "a", target: "/etc/passwd", because: "outside"},
		{entry: "a", target: `C:\windows\system32`, because: "outside"},
		{entry: "a", target: "C:relative", because: "outside"},
		// Backslash-only escapes, which path.Clean alone would not catch because it
		// does not treat a backslash as a separator.
		{entry: "a", target: `..\escape`, because: "outside"},
		{entry: "sub/a", target: `..\..\escape`, because: "outside"},
	} {
		t.Run(test.entry+" -> "+test.target, func(t *testing.T) {
			err := safeLinkTarget(test.entry, test.target)
			if err == nil {
				t.Fatalf("safeLinkTarget(%q, %q) accepted an escape", test.entry, test.target)
			}
			if !strings.Contains(err.Error(), test.because) {
				t.Fatalf("refused with %q, which does not mention %q", err, test.because)
			}
			// The entry is named, because "links outside the archive" without a name
			// does not say which entry to look at in a thousand-entry archive.
			if !strings.Contains(err.Error(), test.entry) {
				t.Fatalf("the message %q does not name the entry %q", err, test.entry)
			}
		})
	}
}

// The collision check, whose whole purpose is a filesystem property the test
// machine may or may not have -- so it is asserted on the logic, not on the disk.
func TestArchiveCollisions_noticesWhatWindowsWouldMerge(t *testing.T) {
	collisions := newArchiveCollisions()
	if err := collisions.check("foo.txt"); err != nil {
		t.Fatalf("the first entry collided with nothing: %v", err)
	}
	// A different case is the same file on NTFS, so the second entry would
	// silently overwrite the first -- a way to smuggle content past a reviewer who
	// read the listing.
	err := collisions.check("FOO.TXT")
	if err == nil {
		t.Fatal("FOO.TXT did not collide with foo.txt")
	}
	if !strings.Contains(err.Error(), "foo.txt") {
		t.Fatalf("the message %q does not name the earlier entry", err)
	}
	// The *same* name twice is not a collision in this sense: it is one entry
	// listed twice, which tar allows and which means "replace".
	if err := collisions.check("bar.txt"); err != nil {
		t.Fatal(err)
	}
	if err := collisions.check("bar.txt"); err != nil {
		t.Fatalf("the same name twice was reported as a collision: %v", err)
	}
	// Different directories, same base name: not a collision.
	if err := collisions.check("a/x.txt"); err != nil {
		t.Fatal(err)
	}
	if err := collisions.check("b/x.txt"); err != nil {
		t.Fatalf("the same base name in two directories collided: %v", err)
	}
}

// Every reserved name in the table is actually refused, walked from the table
// itself so a name added to it without a check cannot go unnoticed.
func TestArchiveReservedNames_everyEntryIsRefused(t *testing.T) {
	if len(archiveReservedNames) != 22 {
		t.Fatalf("the reserved-name table has %d entries; the Windows set is 22 "+
			"(CON PRN AUX NUL, COM1-9, LPT1-9)", len(archiveReservedNames))
	}
	for name := range archiveReservedNames {
		for _, spelling := range []string{name, strings.ToUpper(name), name + ".txt", "sub/" + name} {
			if _, err := safeArchivePath(spelling); err == nil {
				t.Errorf("safeArchivePath(%q) was accepted, but %q is a reserved name", spelling, name)
			}
		}
	}
}

// The property safeArchivePath depends on when it checks for an absolute name
// *before* cleaning and not after.
//
// This test exists because the "after" check was there, was unreachable, and was
// removed. Deleting a guard from security code is only safe if the reason it was
// unreachable is itself checked, so the reason lives here: path.Clean never
// introduces a leading separator into a name that did not have one. If a future Go
// release changed that, this fails and the guard comes back.
func TestPathClean_cannotIntroduceALeadingSeparator(t *testing.T) {
	for _, name := range []string{
		"a", "a/b", "./a", "a/../b", "a/../../b", "..", "../..", "a/..",
		".", "./", "a//b", "a/./b", "...", "..a", "a..", "a/../../../../../../b",
	} {
		if strings.HasPrefix(name, "/") {
			t.Fatalf("the fixture %q is already absolute, so it proves nothing", name)
		}
		if cleaned := path.Clean(name); strings.HasPrefix(cleaned, "/") {
			t.Fatalf("path.Clean(%q) = %q, which begins with a separator: "+
				"safeArchivePath needs its second absolute check back", name, cleaned)
		}
	}
}
