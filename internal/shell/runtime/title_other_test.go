//go:build !windows

package runtime

import (
	"bytes"
	"strings"
	"testing"
)

// Off Windows there is no console title to set or read, and title says so, with 1.
func TestTitle_isRefusedOffWindows(t *testing.T) {
	for _, args := range [][]string{{"x"}, nil} {
		var stdout, stderr bytes.Buffer

		// When
		status := runTitle(args, hostConsoleTitle{}, &stdout, &stderr, "nemosh: ")

		// Then
		if status != 1 || !strings.Contains(stderr.String(), "title: there is no console title here") {
			t.Errorf("title %q: status %d, %q", args, status, stderr.String())
		}
	}
}
