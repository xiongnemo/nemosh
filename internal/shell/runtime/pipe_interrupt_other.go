//go:build !windows

package runtime

import "os"

func interruptPipeIO(_ *os.File) error { return nil }
