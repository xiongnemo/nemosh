// Package osrand is the system's random bytes: RtlGenRandom, advapi32's SystemFunction036, as
// busybox-w32 takes them (win32/mingw.c). crypto/rand would serve as well, and brings the FIPS
// module's entropy source with it, whose 32 MiB buffer every process committed whether or not it
// ever asked for a random byte.
package osrand

import (
	"io"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var genRandom struct {
	once sync.Once
	proc *windows.LazyProc
}

// most is the most one call is asked for: its length is a ULONG, kept clear of the sign bit.
const most = 1<<31 - 1

// Read fills buffer with random bytes. The call does not fail on a Windows nemosh runs on, and if
// it ever does that is returned, never a buffer left as it was.
func Read(buffer []byte) (int, error) {
	genRandom.once.Do(func() { genRandom.proc = windows.NewLazySystemDLL("advapi32.dll").NewProc("SystemFunction036") })
	filled := 0
	for filled < len(buffer) {
		chunk := min(len(buffer)-filled, most)
		if ok, _, err := genRandom.proc.Call(uintptr(unsafe.Pointer(&buffer[filled])), uintptr(chunk)); ok == 0 {
			return filled, err
		}
		filled += chunk
	}
	return filled, nil
}

// Reader is Read as an io.Reader, which never ends.
var Reader io.Reader = reader{}

type reader struct{}

func (reader) Read(buffer []byte) (int, error) { return Read(buffer) }
