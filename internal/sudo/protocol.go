// Package sudo is what passes between `sudo` and the elevated nemosh an SSH session started from.
//
// A nemosh that sshd starts with an administrator's elevated token runs the session's shell with
// that token made unelevated, and stays behind to lend the elevation back: `sudo COMMAND` asks it,
// over a named pipe whose name is in NEMOSH_SUDO, to run COMMAND elevated with the asker's streams,
// directory and environment, and answers with its status. See internal/shell/runtime, ssh_session.
//
// A request is the magic, a word of flags, the asker's three standard handles as numbers in its
// own handle table, which the lender duplicates from it, the directory, the arguments and the
// environment. Strings are a 32-bit length and their bytes, lists a 32-bit count and their strings,
// all little-endian. A reply is the status and a message, said by sudo when it is not empty.
package sudo

import (
	"encoding/binary"
	"fmt"
	"io"
)

const (
	// PipeVariable names the pipe the session's elevated nemosh listens on.
	PipeVariable = "NEMOSH_SUDO"
	// KeepVariable set to keep keeps an SSH session elevated: a user environment variable, since
	// Windows's sshd takes none from the client.
	KeepVariable = "NEMOSH_SSH_ADMIN"
)

// Request is what sudo asks.
type Request struct {
	// Shell asks for a shell rather than a command: sudo -s.
	Shell bool
	// Handles are the asker's stdin, stdout and stderr; 0 is none.
	Handles [3]uint64
	Dir     string
	Args    []string
	Env     []string
}

// Reply is the lender's answer: the command's status, or the reason it did not run.
type Reply struct {
	Status  int32
	Message string
}

const (
	requestMagic = "NSU1"
	flagShell    = 1
	// maxString and maxList bound what is read, so a length from the wire cannot make an
	// allocation of its own size.
	maxString = 1 << 20
	maxList   = 1 << 14
)

// errMalformed is a value of a type of its own rather than errors.New's, which would allocate at
// every start of the shell; see the init ceiling in product.yml.
var errMalformed error = malformedError{}

type malformedError struct{}

func (malformedError) Error() string { return "malformed sudo request" }

// WriteRequest writes request to w.
func WriteRequest(w io.Writer, request Request) error {
	var flags uint32
	if request.Shell {
		flags |= flagShell
	}
	buffer := append([]byte(requestMagic), binary.LittleEndian.AppendUint32(nil, flags)...)
	for _, handle := range request.Handles {
		buffer = binary.LittleEndian.AppendUint64(buffer, handle)
	}
	buffer = appendString(buffer, request.Dir)
	buffer = appendList(appendList(buffer, request.Args), request.Env)
	_, err := w.Write(buffer)
	return err
}

// ReadRequest reads a request from r.
func ReadRequest(r io.Reader) (Request, error) {
	var head [4 + 4 + 24]byte
	if _, err := io.ReadFull(r, head[:]); err != nil {
		return Request{}, fmt.Errorf("%w: %v", errMalformed, err)
	}
	if string(head[:4]) != requestMagic {
		return Request{}, errMalformed
	}
	request := Request{Shell: binary.LittleEndian.Uint32(head[4:])&flagShell != 0}
	for index := range request.Handles {
		request.Handles[index] = binary.LittleEndian.Uint64(head[8+8*index:])
	}
	var err error
	if request.Dir, err = readString(r); err != nil {
		return Request{}, err
	}
	if request.Args, err = readList(r); err != nil {
		return Request{}, err
	}
	if request.Env, err = readList(r); err != nil {
		return Request{}, err
	}
	return request, nil
}

// WriteReply writes reply to w.
func WriteReply(w io.Writer, reply Reply) error {
	buffer := binary.LittleEndian.AppendUint32(nil, uint32(reply.Status))
	_, err := w.Write(appendString(buffer, reply.Message))
	return err
}

// ReadReply reads a reply from r.
func ReadReply(r io.Reader) (Reply, error) {
	var status [4]byte
	if _, err := io.ReadFull(r, status[:]); err != nil {
		return Reply{}, err
	}
	message, err := readString(r)
	return Reply{Status: int32(binary.LittleEndian.Uint32(status[:])), Message: message}, err
}

func appendString(buffer []byte, text string) []byte {
	return append(binary.LittleEndian.AppendUint32(buffer, uint32(len(text))), text...)
}

func appendList(buffer []byte, items []string) []byte {
	buffer = binary.LittleEndian.AppendUint32(buffer, uint32(len(items)))
	for _, item := range items {
		buffer = appendString(buffer, item)
	}
	return buffer
}

func readCount(r io.Reader, limit uint32) (uint32, error) {
	var word [4]byte
	if _, err := io.ReadFull(r, word[:]); err != nil {
		return 0, fmt.Errorf("%w: %v", errMalformed, err)
	}
	count := binary.LittleEndian.Uint32(word[:])
	if count > limit {
		return 0, fmt.Errorf("%w: %d is past its limit", errMalformed, count)
	}
	return count, nil
}

func readString(r io.Reader) (string, error) {
	length, err := readCount(r, maxString)
	if err != nil {
		return "", err
	}
	text := make([]byte, length)
	if _, err := io.ReadFull(r, text); err != nil {
		return "", fmt.Errorf("%w: %v", errMalformed, err)
	}
	return string(text), nil
}

func readList(r io.Reader) ([]string, error) {
	count, err := readCount(r, maxList)
	if err != nil {
		return nil, err
	}
	items := make([]string, 0, count)
	for range count {
		item, err := readString(r)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
