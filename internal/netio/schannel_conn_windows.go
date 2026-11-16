package netio

import (
	"errors"
	"io"
	"sync"
	"time"
	"unsafe"
)

// schannelConn is TLS over plain. Schannel encrypts on one thread while it decrypts on another, so
// Read and Write go on at once, as nc's two directions need; mu is held only for each call into
// Schannel, never while the network is waited on, so a Write stuck on a full peer does not stop a
// Read. A renegotiation -- in TLS 1.3, a session ticket or a key update after the handshake --
// stops both: DecryptMessage has then made the context refuse to encrypt, so Write waits for it.
type schannelConn struct {
	plain      Conn
	credential *secHandle
	target     *uint16
	context    secHandle
	made       bool // the context exists, so it is deleted
	header     int  // STREAM_SIZES: a record's header, its trailer, and the most it carries
	trailer    int
	maximum    int

	readMu    sync.Mutex
	incoming  []byte // read and not yet decrypted
	plaintext []byte // decrypted and not yet read
	decrypted []byte // plaintext's storage
	readErr   error  // the end of the session, or what broke it; a timeout is not kept

	writeMu sync.Mutex // held from a record's encryption until it is sent, so records go in order
	record  []byte

	mu            sync.Mutex
	renegotiating chan struct{} // closed when the renegotiation that made it is done
	deleted       bool
}

func (c *schannelConn) Read(p []byte) (int, error) {
	c.readMu.Lock()
	defer c.readMu.Unlock()
	for len(c.plaintext) == 0 {
		if c.readErr != nil {
			return 0, c.readErr
		}
		if len(c.incoming) > 0 {
			progressed, err := c.decrypt()
			if err != nil {
				c.readErr = err
			}
			if progressed || err != nil {
				continue
			}
		}
		if err := c.fill(); err != nil {
			return 0, err
		}
	}
	n := copy(p, c.plaintext)
	c.plaintext = c.plaintext[n:]
	return n, nil
}

// fill reads more of the session from the connection. An end between records is the end of the
// session, as most servers end one without close_notify; an end within a record is not.
func (c *schannelConn) fill() error {
	if len(c.incoming) == cap(c.incoming) {
		if cap(c.incoming) >= handshakeLimit {
			return errors.New("schannel: a record past 1 MiB")
		}
		grown := max(2*cap(c.incoming), c.header+c.maximum+c.trailer+512)
		c.incoming = append(make([]byte, 0, grown), c.incoming...)
	}
	n, err := c.plain.Read(c.incoming[len(c.incoming):cap(c.incoming)])
	c.incoming = c.incoming[:len(c.incoming)+n]
	switch {
	case n > 0:
		return nil
	case errors.Is(err, io.EOF) && len(c.incoming) == 0:
		c.readErr = io.EOF
	case errors.Is(err, io.EOF):
		c.readErr = io.ErrUnexpectedEOF
	case err == nil:
		return nil
	default:
		return err
	}
	return nil
}

// decrypt is one DecryptMessage of what was read, answering whether it took anything from it.
func (c *schannelConn) decrypt() (bool, error) {
	buffers := []secBuffer{{kind: secBufferData, size: uint32(len(c.incoming)), data: &c.incoming[0]},
		{kind: secBufferEmpty}, {kind: secBufferEmpty}, {kind: secBufferEmpty}}
	c.mu.Lock()
	if c.deleted {
		c.mu.Unlock()
		return false, ErrClosed
	}
	status, _, _ := sspi.decrypt.Call(uintptr(unsafe.Pointer(&c.context)), uintptr(unsafe.Pointer(describe(buffers))), 0, 0)
	if status == secRenegotiate {
		c.renegotiating = make(chan struct{})
	}
	c.mu.Unlock()
	switch status {
	case secIncompleteMessage:
		return false, nil
	case secOK, secRenegotiate, secContextExpired:
	default:
		return false, schannelError("decrypt", status)
	}
	// The first buffer is what was read and becomes the record's header; one it leaves as data is
	// that, undecrypted -- the close_notify record, when the session has ended -- and is not what
	// the record carried.
	extra := 0
	for _, buffer := range buffers[1:] {
		switch buffer.kind {
		case secBufferData:
			if status == secContextExpired {
				continue
			}
			c.decrypted = append(c.decrypted[:0], bytesOf(buffer)...)
			c.plaintext = c.decrypted
		case secBufferExtra:
			extra = int(buffer.size)
		}
	}
	// What is left is at the end of what was read, and is the next record's.
	c.incoming = c.incoming[:copy(c.incoming, c.incoming[len(c.incoming)-extra:])]
	switch status {
	case secContextExpired:
		// close_notify: the server has ended the session.
		return true, io.EOF
	case secRenegotiate:
		return true, c.renegotiate()
	}
	return true, nil
}

// renegotiate runs the handshake again on the context, with Write held off until it is done.
func (c *schannelConn) renegotiate() error {
	defer func() {
		c.mu.Lock()
		close(c.renegotiating)
		c.renegotiating = nil
		c.mu.Unlock()
	}()
	// A Write under way sends its record first; the next sees renegotiating and waits.
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.handshake(c.incoming)
}

func (c *schannelConn) Write(p []byte) (int, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	written := 0
	for len(p) > 0 {
		chunk := min(len(p), c.maximum)
		if need := c.header + c.maximum + c.trailer; len(c.record) < need {
			c.record = make([]byte, need)
		}
		record := c.record
		copy(record[c.header:], p[:chunk])
		buffers := []secBuffer{
			{kind: secBufferStreamHeader, size: uint32(c.header), data: &record[0]},
			{kind: secBufferData, size: uint32(chunk), data: &record[c.header]},
			{kind: secBufferStreamTrailer, size: uint32(c.trailer), data: &record[c.header+chunk]},
			{kind: secBufferEmpty},
		}
		c.mu.Lock()
		if c.deleted {
			c.mu.Unlock()
			return written, ErrClosed
		}
		if wait := c.renegotiating; wait != nil {
			c.mu.Unlock()
			c.writeMu.Unlock()
			<-wait
			c.writeMu.Lock()
			continue
		}
		status, _, _ := sspi.encrypt.Call(uintptr(unsafe.Pointer(&c.context)), 0, uintptr(unsafe.Pointer(describe(buffers))), 0)
		c.mu.Unlock()
		if status != secOK {
			return written, schannelError("encrypt", status)
		}
		// The header, the data and the trailer lie one after another, the trailer perhaps
		// shorter than it might have been.
		if _, err := c.plain.Write(record[:buffers[0].size+buffers[1].size+buffers[2].size]); err != nil {
			return written, err
		}
		written += chunk
		p = p[chunk:]
	}
	return written, nil
}

// CloseWrite sends nothing, as busybox's ssl_client sends nothing when its input ends: TLS can end
// the session, with close_notify, and has no way to end one direction of it, and a server that
// reads close_notify as the end may not answer what it was sent.
func (c *schannelConn) CloseWrite() error { return nil }

// Close closes the connection, which ends a Read or a Write waiting on it, and then the context.
// No close_notify is sent, as busybox sends none.
func (c *schannelConn) Close() error {
	err := c.plain.Close()
	c.release()
	return err
}

func (c *schannelConn) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.made && !c.deleted {
		sspi.deleteContext.Call(uintptr(unsafe.Pointer(&c.context)))
	}
	c.deleted = true
}

func (c *schannelConn) SetReadDeadline(deadline time.Time) error {
	return c.plain.SetReadDeadline(deadline)
}
func (c *schannelConn) SetWriteDeadline(deadline time.Time) error {
	return c.plain.SetWriteDeadline(deadline)
}
func (c *schannelConn) LocalAddr() string  { return c.plain.LocalAddr() }
func (c *schannelConn) RemoteAddr() string { return c.plain.RemoteAddr() }
