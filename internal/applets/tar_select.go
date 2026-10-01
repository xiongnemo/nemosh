package applets

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"strings"
	"syscall"
)

// Which entries tar takes, as busybox selects them.
//
// Listing and extracting take the names given, and an entry under one of them: each name is a
// pattern matched against as many leading components of the entry's name in the archive as the
// pattern has, a `*` crossing a `/` (busybox's find_list_entry2), so `src/sub` takes
// `src/sub/b.log`. --exclude and -X are matched the same way and leave an entry out: `*.log`
// there matches a first component only, as it does in busybox. Both were ignored, and `tar xf
// a.tar one` took everything. --strip-components shortens the names only as they are written.
//
// A name that took nothing is said at the end, and the status is 1, unless an exclusion
// matches the name itself. busybox asks instead whether an entry it took is spelled like the
// name, so a pattern -- `tar xf a.tar 'src/*.txt'` -- fails having extracted what it matched,
// and it says the first name only.
//
// Creating leaves out what an exclusion matches at the start of any component, a `*` stopping
// at a `/` (busybox's exclude_file).
type tarSelection struct {
	accept, reject []string
	strip          int
	took           []bool
}

// leadingMatch is a pattern matched against as many of a name's leading components as it has.
func leadingMatch(pattern, name string) bool {
	keep, cut := strings.Count(pattern, "/"), name
	for index := 0; index < len(name); index++ {
		if name[index] != '/' {
			continue
		}
		if keep == 0 {
			cut = name[:index]
			break
		}
		keep--
	}
	return wholeMatch(pattern, cut)
}

// wholeMatch is fnmatch with no flags: the pattern matches all of the name, a `*` crossing `/`.
func wholeMatch(pattern, name string) bool {
	matcher, err := compileFindPathPattern(pattern)
	return err == nil && matcher.MatchString(name)
}

// takes is whether an entry of the archive is listed or extracted.
func (s *tarSelection) takes(name string) bool {
	for _, pattern := range s.reject {
		if leadingMatch(pattern, name) {
			return false
		}
	}
	if len(s.accept) == 0 {
		return true
	}
	if s.took == nil {
		s.took = make([]bool, len(s.accept))
	}
	for index, pattern := range s.accept {
		if leadingMatch(pattern, name) {
			s.took[index] = true
			return true
		}
	}
	return false
}

// unmatched says each name that took nothing, and answers whether there was one.
func (s *tarSelection) unmatched(stderr io.Writer) bool {
	missing := false
	for index, name := range s.accept {
		if s.took != nil && s.took[index] || s.excluded(name) {
			continue
		}
		fmt.Fprintf(stderr, "tar: %s: not found in archive\n", name)
		missing = true
	}
	return missing
}

// excluded is whether an exclusion matches a name given, so its taking nothing is no failure.
func (s *tarSelection) excluded(name string) bool {
	for _, pattern := range s.reject {
		if wholeMatch(pattern, name) {
			return true
		}
	}
	return false
}

// stripped is a name with --strip-components' leading components taken off, and whether
// anything is left of it.
func (s *tarSelection) stripped(name string) (string, bool) {
	for count := 0; count < s.strip; count++ {
		slash := strings.IndexByte(name, '/')
		if slash < 0 {
			return "", false
		}
		name = name[slash+1:]
	}
	return name, name != ""
}

// strippedEntry takes --strip-components' components off an entry's name, and off a hard
// link's target, and answers whether anything is left of both to extract, as busybox strips
// them: an entry that is all leading components is passed over.
func (s *tarSelection) strippedEntry(header *tar.Header) bool {
	name, ok := s.stripped(header.Name)
	if ok && header.Typeflag == tar.TypeLink {
		header.Linkname, ok = s.stripped(header.Linkname)
	}
	header.Name = name
	return ok
}

// extractFile writes a file entry where it lands, its modification time the archive's but under
// -m, as busybox keeps it. What is there already is removed first, so a link there is replaced
// rather than written through; --overwrite writes into it instead, and -k ends the extraction,
// as busybox's open fails. A directory there is not removed. The file was written through, and
// its time never restored.
func (r tarRequest) extractFile(reader io.Reader, header *tar.Header, name, destination string) error {
	if existing, err := os.Lstat(destination); err == nil && !r.overwrite {
		switch {
		case r.keepOld:
			return cannotOpen(name, fs.ErrExist)
		case existing.IsDir():
			return fmt.Errorf("cannot remove old file %s: %s", name, causeText(syscall.EISDIR))
		}
		if err := os.Remove(destination); err != nil {
			return fmt.Errorf("cannot remove old file %s: %s", name, causeText(err))
		}
	}
	file, err := createFile(r.view, destination)
	if err != nil {
		return cannotOpen(name, err)
	}
	_, copyErr := io.Copy(file, reader)
	if closeErr := file.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr == nil && r.keepTime && !header.ModTime.IsZero() {
		_ = os.Chtimes(destination, header.ModTime, header.ModTime)
	}
	return copyErr
}

// excludedFromArchive is whether creating leaves a name out: an exclusion that matches it from
// the start of any component, and up to the end of one, as busybox's exclude_file matches.
func excludedFromArchive(patterns []string, name string) bool {
	for _, pattern := range patterns {
		for start := 0; start < len(name); start++ {
			if start > 0 && name[start-1] != '/' || name[start] == '/' {
				continue
			}
			if leadingDirMatch(pattern, name[start:]) {
				return true
			}
		}
	}
	return false
}

// leadingDirMatch is fnmatch with FNM_PATHNAME and FNM_LEADING_DIR: the pattern matches the
// whole of the name, or of it up to a `/`.
func leadingDirMatch(pattern, name string) bool {
	for end := 0; end <= len(name); end++ {
		if end < len(name) && name[end] != '/' {
			continue
		}
		if matched, err := path.Match(pattern, name[:end]); err == nil && matched {
			return true
		}
	}
	return false
}
