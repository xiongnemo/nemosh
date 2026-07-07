package runtime

import (
	"crypto/rand"
	"io"
	"os"
)

type zeroReader struct{}

type nullDevice struct{}

func (nullDevice) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (nullDevice) Write(p []byte) (int, error) {
	return len(p), nil
}

func (nullDevice) Close() error {
	return nil
}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func openInputRedirect(path string) (io.ReadCloser, error) {
	switch path {
	case "/dev/null":
		return nullDevice{}, nil
	case "/dev/zero":
		return io.NopCloser(zeroReader{}), nil
	case "/dev/urandom", "/dev/random":
		return io.NopCloser(rand.Reader), nil
	}
	return os.Open(platformPath(path))
}

func openOutputRedirect(path string) (io.WriteCloser, error) {
	if path == "/dev/null" {
		return nullDevice{}, nil
	}
	return os.Create(platformPath(path))
}
