//go:build windows

package sudo

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows"
)

// Ask sends request to the elevated nemosh listening on pipe, and answers with its reply. A pipe
// whose every instance is taken is tried again for a while: the lender opens a new one as soon as
// it has taken an asker.
func Ask(pipe string, request Request) (Reply, error) {
	connection, err := openPipe(pipe)
	if err != nil {
		return Reply{}, fmt.Errorf("cannot reach this session's elevated nemosh: %w", err)
	}
	defer connection.Close()
	if err := WriteRequest(connection, request); err != nil {
		return Reply{}, fmt.Errorf("cannot ask this session's elevated nemosh: %w", err)
	}
	reply, err := ReadReply(connection)
	if err != nil {
		return Reply{}, fmt.Errorf("this session's elevated nemosh did not answer: %w", err)
	}
	return reply, nil
}

func openPipe(pipe string) (*os.File, error) {
	deadline := time.Now().Add(5 * time.Second)
	for {
		connection, err := os.OpenFile(pipe, os.O_RDWR, 0)
		if err == nil || !errors.Is(err, windows.ERROR_PIPE_BUSY) || time.Now().After(deadline) {
			return connection, err
		}
		time.Sleep(20 * time.Millisecond)
	}
}
