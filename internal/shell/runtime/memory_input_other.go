//go:build !windows

package runtime

import (
	"errors"
	"os"
)

// openSharedTemp is a new temporary file with no name left: removed as soon as it is
// open, so it goes once the last descriptor to it closes -- the shell's, and every job's
// it was handed to.
func openSharedTemp() (*os.File, error) {
	file, err := os.CreateTemp("", "nemosh-input-")
	if err != nil {
		return nil, err
	}
	if err := os.Remove(file.Name()); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}
