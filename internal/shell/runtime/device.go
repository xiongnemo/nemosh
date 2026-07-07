package runtime

import (
	"crypto/rand"
	"io"
	"os"
)

type zeroReader struct{}

func (zeroReader) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 0
	}
	return len(p), nil
}

func openInputRedirect(path string) (io.ReadCloser, error) {
	switch path {
	case "/dev/zero":
		return io.NopCloser(zeroReader{}), nil
	case "/dev/urandom", "/dev/random":
		return io.NopCloser(rand.Reader), nil
	}
	return os.Open(platformPath(path))
}
