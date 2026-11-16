package netio

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// TLS is Schannel's, as busybox-w32's is (networking/tls.c): the protocols and cipher suites the
// system has enabled, TLS 1.3 where Windows has it, and no TLS of nemosh's own in the binary.
// Schannel is told to leave the certificate to the caller (SCH_CRED_MANUAL_CRED_VALIDATION), and
// schannel_verify_windows.go asks the system's chain engine about it as crypto/x509 asked it, so
// what is trusted has not changed, and why a certificate is refused can be said.

var sspi struct {
	once                                                              sync.Once
	acquire, initialize, deleteContext, query, encrypt, decrypt, free *windows.LazyProc
}

func sspiLoaded() {
	sspi.once.Do(func() {
		dll := windows.NewLazySystemDLL("secur32.dll")
		sspi.acquire = dll.NewProc("AcquireCredentialsHandleW")
		sspi.initialize = dll.NewProc("InitializeSecurityContextW")
		sspi.deleteContext = dll.NewProc("DeleteSecurityContext")
		sspi.query = dll.NewProc("QueryContextAttributesW")
		sspi.encrypt = dll.NewProc("EncryptMessage")
		sspi.decrypt = dll.NewProc("DecryptMessage")
		sspi.free = dll.NewProc("FreeContextBuffer")
	})
}

const (
	secOK                  = 0x00000000
	secContinueNeeded      = 0x00090312
	secContextExpired      = 0x00090317
	secIncompleteCreds     = 0x00090320
	secRenegotiate         = 0x00090321
	secIncompleteMessage   = 0x80090318
	secBufferEmpty         = 0
	secBufferData          = 1
	secBufferToken         = 2
	secBufferExtra         = 5
	secBufferStreamTrailer = 6
	secBufferStreamHeader  = 7
	secBufferAlert         = 17
	attributeStreamSizes   = 4
	attributeRemoteCert    = 0x53
	// ISC_REQ_SEQUENCE_DETECT, REPLAY_DETECT, CONFIDENTIALITY, ALLOCATE_MEMORY, USE_SUPPLIED_CREDS,
	// STREAM and MANUAL_CRED_VALIDATION.
	contextRequest = 0x8 | 0x4 | 0x10 | 0x100 | 0x80 | 0x8000 | 0x80000
	// handshakeLimit is how much of a handshake is held while waiting for the rest of a message.
	handshakeLimit = 1 << 20
)

type secHandle struct{ lower, upper uintptr }

type secBuffer struct {
	size, kind uint32
	data       *byte
}

type secBufferDesc struct {
	version, count uint32
	buffers        *secBuffer
}

// describe is a SecBufferDesc for buffers. It is converted to a uintptr only in the call that takes
// it, so it is kept alive until the call returns.
func describe(buffers []secBuffer) *secBufferDesc {
	return &secBufferDesc{count: uint32(len(buffers)), buffers: &buffers[0]}
}

// bytesOf is what an SSPI buffer holds, which may be Schannel's memory or ours.
func bytesOf(buffer secBuffer) []byte {
	if buffer.data == nil || buffer.size == 0 {
		return nil
	}
	return unsafe.Slice(buffer.data, buffer.size)
}

// schannelError is a status Schannel answered, with the system's words for it.
func schannelError(what string, status uintptr) error {
	return fmt.Errorf("schannel: %s: (%#x) %s", what, uint32(status), windows.Errno(status).Error())
}

// credentials is the process's one Schannel credential, outbound and with no certificate of its
// own: SCH_CREDENTIALS, which Windows 10 1809 and later take and which is what lets TLS 1.3 be
// negotiated, and SCHANNEL_CRED where that is refused.
// It is made on first use, as the DLL is loaded, so a process that speaks no TLS pays nothing.
func credentials() (*secHandle, error) {
	acquired.once.Do(func() { acquired.handle, acquired.err = acquireCredentials() })
	return acquired.handle, acquired.err
}

var acquired struct {
	once   sync.Once
	handle *secHandle
	err    error
}

func acquireCredentials() (*secHandle, error) {
	sspiLoaded()
	// SCH_CRED_MANUAL_CRED_VALIDATION, SCH_CRED_NO_DEFAULT_CREDS and SCH_USE_STRONG_CRYPTO.
	const flags = 0x8 | 0x10 | 0x00400000
	modern := struct {
		version, format, creds uint32
		certificates, root     uintptr
		mappers                uint32
		mapperList             uintptr
		lifespan, flags, count uint32
		parameters             uintptr
	}{version: 5, flags: flags}
	legacy := struct {
		version, creds              uint32
		certificates, root          uintptr
		mappers                     uint32
		mapperList                  uintptr
		algorithms                  uint32
		algorithmList               uintptr
		protocols, minimum, maximum uint32
		lifespan, flags, format     uint32
	}{version: 4, flags: flags}
	name, _ := windows.UTF16PtrFromString("Microsoft Unified Security Protocol Provider")
	handle := new(secHandle)
	var expiry int64
	status := uintptr(0)
	for _, data := range []unsafe.Pointer{unsafe.Pointer(&modern), unsafe.Pointer(&legacy)} {
		const outbound = 2
		status, _, _ = sspi.acquire.Call(0, uintptr(unsafe.Pointer(name)), outbound, 0, uintptr(data), 0, 0,
			uintptr(unsafe.Pointer(handle)), uintptr(unsafe.Pointer(&expiry)))
		if status == secOK {
			return handle, nil
		}
	}
	return nil, schannelError("no credentials", status)
}

// Client is TLS over conn to serverName, its certificate verified against the system's roots.
func Client(conn Conn, serverName string) (Conn, error) {
	credential, err := credentials()
	if err != nil {
		return nil, err
	}
	target, err := windows.UTF16PtrFromString(serverName)
	if err != nil {
		return nil, err
	}
	c := &schannelConn{plain: conn, credential: credential, target: target}
	if err := c.handshake(nil); err != nil {
		c.release()
		return nil, err
	}
	if err := c.verify(serverName); err != nil {
		c.release()
		return nil, err
	}
	var sizes [5]uint32
	if status, _, _ := sspi.query.Call(uintptr(unsafe.Pointer(&c.context)), attributeStreamSizes, uintptr(unsafe.Pointer(&sizes))); status != secOK {
		c.release()
		return nil, schannelError("no stream sizes", status)
	}
	c.header, c.trailer, c.maximum = int(sizes[0]), int(sizes[1]), int(sizes[2])
	return c, nil
}

// handshake calls InitializeSecurityContextW until the context is made, sending what it makes
// and reading what it needs, as curl's schannel.c does. The first call of a connection has
// nothing to read, being the ClientHello; a renegotiation begins with incoming, what
// DecryptMessage left. What is left after the handshake is application data, kept for Read.
func (c *schannelConn) handshake(incoming []byte) error {
	buffer := append(make([]byte, 0, 16<<10), incoming...)
	read := false
	for {
		if read {
			if len(buffer) == cap(buffer) {
				if cap(buffer) >= handshakeLimit {
					return errors.New("schannel: a handshake message past 1 MiB")
				}
				buffer = append(buffer, make([]byte, cap(buffer))...)[:len(buffer)]
			}
			n, err := c.plain.Read(buffer[len(buffer):cap(buffer)])
			buffer = buffer[:len(buffer)+n]
			if n == 0 && err != nil {
				return fmt.Errorf("schannel: handshake: %w", err)
			}
		}
		read = true
		// Schannel may decrypt a handshake record where it lies, so it is given a copy, and the
		// bytes read stay as they came for the call after an incomplete one.
		token := make([]byte, len(buffer), len(buffer)+1)
		copy(token, buffer)
		input := []secBuffer{{kind: secBufferToken, size: uint32(len(token)), data: &token[:1][0]}, {kind: secBufferEmpty}}
		output := []secBuffer{{kind: secBufferToken}, {kind: secBufferAlert}, {kind: secBufferEmpty}}
		status, err := c.initialize(input, output)
		if err != nil {
			return err
		}
		sendErr := c.send(output, status == secOK || status == secContinueNeeded)
		switch {
		case status == secIncompleteMessage:
			continue
		case status == secIncompleteCreds:
			// The server asked for a certificate and there is none: go on without one, and let
			// the server decide.
			read = false
			continue
		case status != secOK && status != secContinueNeeded:
			return schannelError("handshake failed", status)
		case sendErr != nil:
			return fmt.Errorf("schannel: handshake: %w", sendErr)
		}
		if input[1].kind == secBufferExtra && input[1].size > 0 {
			buffer = append(buffer[:0], buffer[len(buffer)-int(input[1].size):]...)
			read = false
		} else {
			buffer = buffer[:0]
		}
		if status == secOK {
			c.incoming = append(c.incoming[:0], buffer...)
			return nil
		}
	}
}

// initialize is one InitializeSecurityContextW: the first makes the context, the rest continue it.
func (c *schannelConn) initialize(input, output []secBuffer) (uintptr, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.deleted {
		return 0, ErrClosed
	}
	var attributes uint32
	var expiry int64
	context, created, inputs := &c.context, (*secHandle)(nil), describe(input)
	if !c.made {
		context, created, inputs = nil, &c.context, nil
	}
	status, _, _ := sspi.initialize.Call(uintptr(unsafe.Pointer(c.credential)), uintptr(unsafe.Pointer(context)),
		uintptr(unsafe.Pointer(c.target)), contextRequest, 0, 0, uintptr(unsafe.Pointer(inputs)), 0,
		uintptr(unsafe.Pointer(created)), uintptr(unsafe.Pointer(describe(output))),
		uintptr(unsafe.Pointer(&attributes)), uintptr(unsafe.Pointer(&expiry)))
	if !c.made && (status == secOK || status == secContinueNeeded) {
		c.made = true
	}
	return status, nil
}

// send writes the tokens Schannel made, or, when it failed, the alert it made, and frees them.
func (c *schannelConn) send(output []secBuffer, succeeded bool) error {
	var err error
	for _, buffer := range output {
		wanted := buffer.kind == secBufferToken && succeeded || buffer.kind == secBufferAlert && !succeeded
		if data := bytesOf(buffer); wanted && len(data) > 0 && err == nil {
			_, err = c.plain.Write(data)
		}
		if buffer.data != nil {
			sspi.free.Call(uintptr(unsafe.Pointer(buffer.data)))
		}
	}
	return err
}
