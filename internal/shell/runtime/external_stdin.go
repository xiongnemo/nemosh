package runtime

import (
	"context"
	"io"
	"os"
)

type stdinFileLeaser interface {
	LeaseStdinFile(context.Context) (*os.File, func(), bool)
}

func externalStdin(ctx context.Context, input io.Reader) (io.Reader, func()) {
	if file, ok := input.(*os.File); ok {
		return file, func() {}
	}
	leaser, ok := input.(stdinFileLeaser)
	if !ok {
		return input, func() {}
	}
	file, release, available := leaser.LeaseStdinFile(ctx)
	if !available {
		return input, func() {}
	}
	return file, release
}
