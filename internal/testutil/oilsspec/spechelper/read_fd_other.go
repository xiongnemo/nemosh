//go:build !windows

package main

import "os"

// readFD reads once from an inherited descriptor, at most 1024 bytes, as os.read(fd, 1024)
// does.
func readFD(fd int) ([]byte, error) {
	file := os.NewFile(uintptr(fd), "fd")
	buffer := make([]byte, 1024)
	n, err := file.Read(buffer)
	if n == 0 && err != nil {
		return nil, err
	}
	return buffer[:n], nil
}
