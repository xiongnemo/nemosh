package applets

import (
	"compress/gzip"
	"io"

	dsbzip2 "github.com/dsnet/compress/bzip2"
)

// compressor is a writer that compresses into out with codec, at level 1 to 9, or -1 for the
// codec's own default: gzip's 6, and bzip2's 9, as the bzip2 program and busybox's take it.
// bzip2 is dsnet/compress's, the standard library having only a reader; its output is the
// format's, and any bunzip2 reads it, though not byte for byte busybox's.
func compressor(out io.Writer, codec string, level int) (io.WriteCloser, error) {
	switch codec {
	case "xz", "lzma":
		return xzCompressor(out, codec)
	}
	if codec == "bzip2" {
		if level < 1 {
			level = 9
		}
		return dsbzip2.NewWriter(out, &dsbzip2.WriterConfig{Level: level})
	}
	return gzip.NewWriterLevel(out, level)
}
