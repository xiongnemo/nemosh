//go:build !windows

package runtime

import "errors"

// hostConsoleTitle has nothing to ask off Windows: a terminal's title can be set with an escape
// but never read back, and an escape written where the output is redirected lands in a file.
type hostConsoleTitle struct{}

var errNoConsoleTitle = errors.New("there is no console title here; it is Windows'")

func (hostConsoleTitle) setTitle(string) error  { return errNoConsoleTitle }
func (hostConsoleTitle) title() (string, error) { return "", errNoConsoleTitle }
