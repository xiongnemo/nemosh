package runtime

import (
	"io"
	"os"
)

// nativeWriter finds the *os.File underneath a stream, or nil when the stream
// is capturing into memory or a pipe this process owns.
//
// It exists so an external command can be handed the real handle. os/exec
// inherits a handle only when the field holds an *os.File; anything else makes
// it open a pipe and copy. The synchronized wrapper this shell puts around
// stdout and stderr is exactly that "anything else", so every child was being
// given a pipe.
//
// The consequences are not about speed. A child asks the handle what it is:
//
//   - `help.exe` writes through the console API when it has a console, and
//     writes code-page bytes when it has a pipe. Those bytes then reached Go's
//     console writer, which decodes UTF-8, so CP936 output on a Chinese Windows
//     arrived as replacement characters. busybox-w32 never had this because it
//     lets the child inherit the console.
//   - Anything checking isatty -- colours, progress bars, pagers -- turned
//     itself off.
//
// The mutex only ever ordered this shell's own writes. A child writing straight
// to the terminal is what every shell does, so handing the handle over loses
// nothing that was being relied on.
// Three layers sit between the runtime and the file, and all three have to be
// walked: the descriptor indirection the FD table hands out, the open
// description a descriptor points at, and the mutex wrapper around the original
// stream.
func nativeWriter(writer io.Writer) io.Writer {
	for range maxWriterUnwrapDepth {
		switch current := writer.(type) {
		case nil:
			return nil
		case *os.File:
			return current
		case synchronizedWriter:
			writer = current.writer
		case descriptorWriter:
			resolved, err := current.table.writer(current.fd)
			if err != nil {
				return nil
			}
			writer = resolved
		case *openDescription:
			// Only when the description is not the owner. An owned resource is
			// closed by the table, and handing it to a child would let the
			// child's exit and the table's close race for it.
			if current.closer != nil {
				return nil
			}
			writer = current.writer
		default:
			return nil
		}
	}
	return nil
}

// maxWriterUnwrapDepth bounds the walk. The layers are finite by construction,
// but a descriptor that resolved to itself would otherwise spin forever.
const maxWriterUnwrapDepth = 8

// nativeReader is the same question for the input side, so a child that prompts
// for a password or draws a full-screen interface reads the console rather than
// a pipe this shell is copying into.
func nativeReader(reader io.Reader) io.Reader {
	if file, ok := reader.(*os.File); ok {
		return file
	}
	return nil
}
