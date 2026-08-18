package runtime

import (
	"bytes"
	"io"
	"sync"
)

// Stream defaults and the writer that serialises them. Split from runtime.go to stay
// under the 250-line ceiling when `declare` was added to the dispatch; nothing else
// moved with them.

func fillStreams(streams Streams) Streams {
	if streams.Stdin == nil {
		streams.Stdin = bytes.NewReader(nil)
	}
	if streams.Stdout == nil {
		streams.Stdout = io.Discard
	}
	if streams.Stderr == nil {
		streams.Stderr = io.Discard
	}
	mutex := &sync.Mutex{}
	streams.Stdout = synchronizedWriter{mutex: mutex, writer: streams.Stdout}
	streams.Stderr = synchronizedWriter{mutex: mutex, writer: streams.Stderr}
	return streams
}

type synchronizedWriter struct {
	mutex  *sync.Mutex
	writer io.Writer
}

func (w synchronizedWriter) Write(buffer []byte) (int, error) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.writer.Write(buffer)
}
