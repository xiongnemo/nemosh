package applets

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// What `mktemp` gets wrong is worth stating, because three of the four defects were silent.
//
//   - `mktemp fooXXXXXX` created the file in the *temporary directory*. Both references
//     create it in the current one: a template without a directory part is a relative path,
//     and only `-t`, `-p` or no template at all reaches for $TMPDIR. A script doing
//     `f=$(mktemp out.XXXXXX)` got a file somewhere else entirely and no indication of it.
//   - `mktemp noxes` and `mktemp aXXX` both succeeded, inventing a suffix from nothing. The
//     template's X run was never read, only trimmed.
//   - the replacement was Go's, so `fooXXXXXX` -- six X -- came back as `foo3524066703`,
//     ten digits. The count is part of the template.
//   - `-t` and `-p DIR` were refused outright, and busybox-w32 has both.
//
// The X rule here is GNU's, not busybox's, and that is the one deliberate divergence from the
// primary reference: busybox demands the template *end* with exactly `XXXXXX` and refuses
// `aXXX`, while GNU and uutils want at least three. GNU's rule accepts everything busybox
// accepts, so no script that works against the reference fails here, and the reverse choice
// would have broken GNU-written scripts for nothing. busybox's diagnostic is not copied
// either -- it reads `mktemp: : Invalid argument`, naming no operand at all.

const (
	// temporaryNameAttempts bounds the retry when a generated name is taken. os.CreateTemp
	// uses 10000 for the same reason; a bound matters because a full directory would
	// otherwise spin.
	temporaryNameAttempts = 10000
	// leastTemplateXs is GNU's minimum. Fewer is a diagnostic, not a name.
	leastTemplateXs       = 3
	temporaryNameAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
)

// temporaryTemplate is a template split into the parts that decide the name.
type temporaryTemplate struct {
	// directory is the operand's own spelling, which is what the answer is built from:
	// both references print the path the template asked for rather than a resolved one,
	// so `mktemp fooXXXXXX` answers `fooa1b2c3` and not an absolute path.
	directory string
	// hostDirectory is where the name is really created. It is not always the same
	// place: `cd` in this shell moves the shell's view of the working directory and not
	// the process's, so creating a relative name without resolving it first put the file
	// wherever the process happened to be. Measured -- in the corpus sandbox the file
	// landed outside the case's own directory and the case could not find it.
	hostDirectory string
	prefix        string
	suffix        string
	// count is how many X the template asked for, and therefore how many random
	// characters replace them.
	count int
}

// parseTemporaryTemplate splits a template at its trailing X run.
//
// The run has to be the last one in the final component: `aXXXXXX.txt` names a suffix, which
// GNU accepts only with --suffix and busybox refuses outright, so it is refused here too.
func parseTemporaryTemplate(template string) (temporaryTemplate, error) {
	directory, base := filepath.Split(filepath.FromSlash(template))
	trimmed := strings.TrimRight(base, "X")
	count := len(base) - len(trimmed)
	if count < leastTemplateXs {
		return temporaryTemplate{}, fmt.Errorf("too few X's in template '%s'", template)
	}
	return temporaryTemplate{directory: directory, prefix: trimmed, count: count}, nil
}

// createTemporary makes the file or directory the template names, returning its path.
//
// Creation is what guarantees uniqueness: a name that is merely unused when it is chosen is a
// race. O_EXCL and Mkdir both fail if the name appeared in between, so a collision retries.
func createTemporary(spec temporaryTemplate, wantDirectory, nameOnly bool) (string, error) {
	for attempt := 0; attempt < temporaryNameAttempts; attempt++ {
		name := spec.prefix + randomTemporaryText(spec.count) + spec.suffix
		answer := filepath.Join(spec.directory, name)
		if nameOnly {
			// -u is unsafe by nature: nothing is reserved, so there is nothing to
			// retry on either.
			return answer, nil
		}
		if err := reserveTemporary(filepath.Join(spec.hostDirectory, name), wantDirectory); err == nil {
			return answer, nil
		} else if !os.IsExist(err) {
			return "", err
		}
	}
	return "", fmt.Errorf("could not find an unused name in %d attempts", temporaryNameAttempts)
}

func reserveTemporary(path string, wantDirectory bool) error {
	if wantDirectory {
		return os.Mkdir(path, 0o700)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	return file.Close()
}

// randomTemporaryText is count characters from crypto/rand.
//
// crypto/rand rather than math/rand because the whole point of the applet is a name nobody
// else can predict, and it is already linked into this binary.
func randomTemporaryText(count int) string {
	bytes := make([]byte, count)
	if _, err := rand.Read(bytes); err != nil {
		// crypto/rand.Read is documented never to fail; if it ever does, a panic here
		// beats a predictable name.
		panic("mktemp: " + err.Error())
	}
	for index, value := range bytes {
		bytes[index] = temporaryNameAlphabet[int(value)%len(temporaryNameAlphabet)]
	}
	return string(bytes)
}
