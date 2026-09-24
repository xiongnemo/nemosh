package runtime

import (
	"bytes"
	"errors"
	"io"
	"os"
	"sync"
)

// memoryInput is text a descriptor reads from memory: a heredoc, a here-string, the
// clipboard. It stays in memory until a job process is given the descriptor. Then what is
// left of it moves to a temporary file, which the shell reads from as well. The two then
// share one offset, as the references' do, because their heredoc is a file (or a pipe)
// from the start: what the job reads, the shell does not read again.
type memoryInput struct{ *memorySource }

type memorySource struct {
	mu     sync.Mutex
	reader *bytes.Reader
	file   *os.File
}

func newMemoryInput(text []byte) memoryInput {
	return memoryInput{&memorySource{reader: bytes.NewReader(text)}}
}

func (m memoryInput) Read(buffer []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file != nil {
		return m.file.Read(buffer)
	}
	return m.reader.Read(buffer)
}

func (m memoryInput) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file == nil {
		return nil
	}
	err := m.file.Close()
	m.file = nil
	m.reader = bytes.NewReader(nil)
	return err
}

// share is the file the text now lives in, made the first time it is asked for.
func (m memoryInput) share() (*os.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.file != nil {
		return m.file, nil
	}
	file, err := openSharedTemp()
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(file, m.reader); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	m.file = file
	return file, nil
}
