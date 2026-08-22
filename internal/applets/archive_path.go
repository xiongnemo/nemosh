package applets

import (
	"fmt"
	"path"
	"strings"
)

// Where an archive entry is allowed to land.
//
// This is the most security-relevant code in the applet set, and the Windows
// hazard list is longer than the Unix one. An archive is untrusted input that
// names its own destinations, so every name is checked before anything is
// created. `docs/testing/applet-test-inventory.md` names "path traversal safety"
// as this group's test focus.
//
// Shared by tar, unzip, cpio, ar and httpd, because a containment rule that
// exists in five copies is five rules eventually.

// archiveReservedNames are the device names Windows resolves in *every*
// directory. Extracting a file called `NUL` writes to the null device and
// silently loses the data -- and `docs/design/device-filesystem.md` already
// records that `test -e ./NUL` succeeds anywhere on this platform, so this is a
// live hazard rather than a theoretical one.
var archiveReservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true, "com5": true,
	"com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true, "lpt5": true,
	"lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// safeArchivePath checks one entry name and returns the slash-separated path it
// may be created at, relative to the extraction root.
//
// Every rejection names its reason, because "refused" without a cause is
// indistinguishable from a bug in the archive reader.
func safeArchivePath(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty entry name")
	}
	// `C:\x`, `C:/x` and the drive-relative `C:x`, which resolves against the
	// *current directory of that drive* and is the sneakiest of the three.
	// Checked before separators are normalised, since normalising does not change
	// the colon.
	if len(name) >= 2 && name[1] == ':' {
		return "", fmt.Errorf("drive-qualified entry name: %s", name)
	}
	// A backslash is normalised to a separator rather than refused, and that is
	// an interoperability decision with a measurement behind it: **PowerShell's
	// Compress-Archive writes backslash-separated names** -- `src\a.txt` --
	// against the zip specification, which mandates `/`. Refusing them made every
	// PowerShell-made zip completely unextractable, and PowerShell is the most
	// likely producer of a zip on this platform.
	//
	// Normalising is not a weakening. `a\..\..\evil` becomes `a/../../evil` and is
	// still caught below by the escape check, which is where the real work
	// happens. What it costs is that a Unix file *literally* named `a\b` becomes
	// `a/b` -- a rare misreading with no security consequence, and the same thing
	// every Windows unzip does.
	name = strings.ReplaceAll(name, `\`, "/")
	if strings.HasPrefix(name, "/") {
		return "", fmt.Errorf("absolute entry name: %s", name)
	}
	cleaned := path.Clean(name)
	if cleaned == "." || cleaned == "/" {
		return "", fmt.Errorf("entry name resolves to the root: %s", name)
	}
	// After cleaning, any remaining `..` escapes. Checking before cleaning would
	// miss `a/b/../../../x`, and checking only the prefix would miss it too.
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("entry name escapes the archive root: %s", name)
	}
	// There is no second absolute check here, and that is deliberate rather than an
	// omission. A leading separator is refused above, *before* cleaning, and
	// path.Clean cannot introduce one into a name that did not have one -- so a
	// check here would be unreachable, and an unreachable guard in security code
	// reads as though it were doing work. The property it would have covered is
	// pinned by a test instead: TestPathClean_cannotIntroduceALeadingSeparator.
	for _, element := range strings.Split(cleaned, "/") {
		if err := checkArchiveElement(element, name); err != nil {
			return "", err
		}
	}
	return cleaned, nil
}

func checkArchiveElement(element, name string) error {
	if element == "" || element == "." || element == ".." {
		return fmt.Errorf("entry name has an empty or relative component: %s", name)
	}
	// Windows silently strips a trailing dot or space, so `evil.` and `evil`
	// become the same file -- which lets an archive smuggle a second entry past a
	// name check that compared strings.
	if strings.HasSuffix(element, ".") || strings.HasSuffix(element, " ") {
		return fmt.Errorf("entry name component ends in a dot or space: %s", name)
	}
	// The reserved name applies with or without an extension: `NUL`, `nul.txt`
	// and `NUL.tar.gz` all reach the device.
	stem, _, _ := strings.Cut(strings.ToLower(element), ".")
	if archiveReservedNames[stem] {
		return fmt.Errorf("entry name is a reserved device name: %s", name)
	}
	return nil
}

// safeLinkTarget checks where a symlink or hardlink entry points.
//
// A link is the second way out of the root: an entry `a -> ../../etc/passwd` is
// harmless in itself, but a later entry writing *through* `a` escapes. So the
// target is held to the same rule as a name, and an absolute one is refused
// outright.
func safeLinkTarget(entryName, target string) error {
	if target == "" {
		return fmt.Errorf("entry %s has an empty link target", entryName)
	}
	if strings.HasPrefix(target, "/") || (len(target) >= 2 && target[1] == ':') {
		return fmt.Errorf("entry %s links outside the archive: %s", entryName, target)
	}
	// Normalised for the same reason an entry name is, then held to the same
	// rule: after this the escape check below does the work.
	target = strings.ReplaceAll(target, `\`, "/")
	// Resolved against the link's own directory, because that is where a relative
	// target is followed from.
	resolved := path.Clean(path.Join(path.Dir(entryName), target))
	if resolved == ".." || strings.HasPrefix(resolved, "../") {
		return fmt.Errorf("entry %s links outside the archive: %s", entryName, target)
	}
	return nil
}

// archiveCollisions notices two entries that Windows would treat as one file.
//
// NTFS is case-insensitive by default, so an archive holding both `FOO` and `foo`
// extracts as one file with the second silently overwriting the first. That is a
// way to smuggle content past a review that read the listing.
type archiveCollisions struct{ seen map[string]string }

func newArchiveCollisions() *archiveCollisions {
	return &archiveCollisions{seen: map[string]string{}}
}

func (c *archiveCollisions) check(name string) error {
	folded := strings.ToLower(name)
	if previous, found := c.seen[folded]; found && previous != name {
		return fmt.Errorf("entry %s collides with %s on a case-insensitive filesystem", name, previous)
	}
	c.seen[folded] = name
	return nil
}
