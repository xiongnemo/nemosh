package runtime

import (
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
	if path == "/dev/zero" {
		return io.NopCloser(zeroReader{}), nil
	}
	return os.Open(platformPath(path))
}
