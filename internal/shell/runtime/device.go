package runtime

import (
	"crypto/rand"
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

func openInputRedirect(path string, streams Streams) (io.ReadCloser, error) {
	switch path {
	case "/dev/null":
		return nullDevice{}, nil
	case "/dev/stdin":
		return readNoopCloser{Reader: streams.Stdin}, nil
	case "/dev/zero":
		return io.NopCloser(zeroReader{}), nil
	case "/dev/urandom", "/dev/random":
		return io.NopCloser(rand.Reader), nil
	case "/dev/clipboard":
		return openClipboardReader()
	}
	return os.Open(platformPath(path))
}

func openInputDevice(path string) (io.ReadCloser, error) {
	switch path {
	case "/dev/null":
		return nullDevice{}, nil
	case "/dev/zero":
		return io.NopCloser(zeroReader{}), nil
	case "/dev/urandom", "/dev/random":
		return io.NopCloser(rand.Reader), nil
	case "/dev/clipboard":
		return openClipboardReader()
	default:
		return nil, fmt.Errorf("%s: %w", path, errUnsupportedDevice)
	}
}

// appendMode reaches this far because /dev/clipboard is the one device where
// `>>` differs from `>`; the others hold nothing to append to.
func openOutputDevice(path string, appendMode bool) (io.WriteCloser, error) {
	switch path {
	case "/dev/null":
		return nullDevice{}, nil
	case "/dev/clipboard":
		return openClipboardWriter(appendMode)
	default:
		return nil, fmt.Errorf("%s: %w", path, errUnsupportedDevice)
	}
}
