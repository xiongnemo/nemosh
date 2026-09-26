package main

import "errors"

// readFD refuses on Windows. A process there is handed its three standard handles and
// nothing numbered past them; the C runtime's own scheme for passing more is not one a Go
// program reads. The number is not tried as a handle either: the kernel ignores a
// handle's low two bits, so 5 would read whatever handle 4 happens to be.
func readFD(int) ([]byte, error) {
	return nil, errors.New("a Windows process has no descriptor past 2")
}
