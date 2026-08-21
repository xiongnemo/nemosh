package runtime

import (
	"fmt"
	"io"
	"os"
)

type zeroReader struct{}

type nullDevice struct{}

type readNoopCloser struct {
	io.Reader
}

type writeNoopCloser struct {
	io.Writer
}

func (nullDevice) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (nullDevice) Write(p []byte) (int, error) {
	return len(p), nil
}

func (nullDevice) Close() error {
	return nil
}

func (readNoopCloser) Close() error {
	return nil
}

func (writeNoopCloser) Close() error {
	return nil
}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

// openInputRedirect opens `< path`, where path may be a device or an ordinary file.
//
// `/dev/stdin` is answered here rather than from the table because it is not a device with contents:
// it is this process's own descriptor, and the streams to hand it back are only available on this
// path. See device_table.go for why the descriptor aliases are kept out of the table.
func openInputRedirect(path string, streams Streams) (io.ReadCloser, error) {
	if path == "/dev/stdin" {
		return readNoopCloser{Reader: streams.Stdin}, nil
	}
	if device, found := lookupDevice(path); found && device.openRead != nil {
		return device.openRead()
	}
	return os.Open(platformPath(path))
}

func openInputDevice(path string) (io.ReadCloser, error) {
	device, found := lookupDevice(path)
	if !found || device.openRead == nil {
		return nil, fmt.Errorf("%s: %w", path, errUnsupportedDevice)
	}
	return device.openRead()
}

// appendMode reaches this far because /dev/clipboard is the one device where
// `>>` differs from `>`; the others hold nothing to append to.
func openOutputDevice(path string, appendMode bool) (io.WriteCloser, error) {
	device, found := lookupDevice(path)
	if !found || device.openWrite == nil {
		return nil, fmt.Errorf("%s: %w", path, errUnsupportedDevice)
	}
	return device.openWrite(appendMode)
}
