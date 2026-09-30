//go:build !windows

package applets

import "syscall"

// syncFilesystems is sync(2): every filesystem's buffered blocks, written.
func syncFilesystems() { syscall.Sync() }

// isFlushRefusal is only Windows' question; a flush here fails for a reason of its own.
func isFlushRefusal(error) bool { return false }
