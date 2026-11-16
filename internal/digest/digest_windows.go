// Package digest is the system's hashes, CNG's, as busybox-w32's md5sum and shaNsum are when it
// is built with the CNG API (libbb/hash_md5_sha.c): MD5, SHA-1, SHA-256, SHA-384 and SHA-512.
// Go's own would serve as well, and brought the FIPS module with them, whose entropy source
// committed 32 MiB in every process whether a hash was asked for or not. SHA-3 stays Go's: CNG has
// no SHA3-224, and busybox's SHA-3 is its own code too.
package digest

import (
	"fmt"
	"hash"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var bcrypt struct {
	once                                           sync.Once
	open, create, data, finish, duplicate, destroy *windows.LazyProc
}

func bcryptLoaded() {
	bcrypt.once.Do(func() {
		dll := windows.NewLazySystemDLL("bcrypt.dll")
		bcrypt.open = dll.NewProc("BCryptOpenAlgorithmProvider")
		bcrypt.create = dll.NewProc("BCryptCreateHash")
		bcrypt.data = dll.NewProc("BCryptHashData")
		bcrypt.finish = dll.NewProc("BCryptFinishHash")
		bcrypt.duplicate = dll.NewProc("BCryptDuplicateHash")
		bcrypt.destroy = dll.NewProc("BCryptDestroyHash")
	})
}

// algorithm is one of CNG's, its provider opened the first time it is wanted and kept, as busybox
// keeps it.
type algorithm struct {
	name        string // its BCRYPT_*_ALGORITHM identifier
	size, block int
	once        sync.Once
	provider    uintptr
	err         error
}

var (
	md5Algorithm    = algorithm{name: "MD5", size: 16, block: 64}
	sha1Algorithm   = algorithm{name: "SHA1", size: 20, block: 64}
	sha256Algorithm = algorithm{name: "SHA256", size: 32, block: 64}
	sha384Algorithm = algorithm{name: "SHA384", size: 48, block: 128}
	sha512Algorithm = algorithm{name: "SHA512", size: 64, block: 128}
)

// MD5, SHA1, SHA256, SHA384 and SHA512 are new hashes, as crypto's New functions are.
func MD5() hash.Hash    { return newDigest(&md5Algorithm) }
func SHA1() hash.Hash   { return newDigest(&sha1Algorithm) }
func SHA256() hash.Hash { return newDigest(&sha256Algorithm) }
func SHA384() hash.Hash { return newDigest(&sha384Algorithm) }
func SHA512() hash.Hash { return newDigest(&sha512Algorithm) }

func (a *algorithm) opened() uintptr {
	a.once.Do(func() {
		bcryptLoaded()
		name, _ := windows.UTF16PtrFromString(a.name)
		if status, _, _ := bcrypt.open.Call(uintptr(unsafe.Pointer(&a.provider)), uintptr(unsafe.Pointer(name)), 0, 0); status != 0 {
			a.err = fmt.Errorf("digest: BCryptOpenAlgorithmProvider(%s): %#x", a.name, uint32(status))
		}
	})
	if a.err != nil {
		// Every Windows since Vista has these, so there is no caller to hand this to: a hash
		// cannot say it failed.
		panic(a.err)
	}
	return a.provider
}

// digest is a hash.Hash over a CNG hash object, which is destroyed when the digest is collected.
type digest struct {
	algorithm *algorithm
	object    uintptr
	cleanup   runtime.Cleanup
}

func newDigest(a *algorithm) *digest {
	d := &digest{algorithm: a}
	d.Reset()
	return d
}

func created(provider uintptr) uintptr {
	var object uintptr
	if status, _, _ := bcrypt.create.Call(provider, uintptr(unsafe.Pointer(&object)), 0, 0, 0, 0, 0); status != 0 {
		panic(fmt.Errorf("digest: BCryptCreateHash: %#x", uint32(status)))
	}
	return object
}

func destroyed(object uintptr) { bcrypt.destroy.Call(object) }

func (d *digest) Reset() {
	if d.object != 0 {
		d.cleanup.Stop()
		destroyed(d.object)
	}
	d.object = created(d.algorithm.opened())
	d.cleanup = runtime.AddCleanup(d, destroyed, d.object)
}

// most is the most one BCryptHashData is given: its length is a ULONG.
const most = 1 << 30

func (d *digest) Write(data []byte) (int, error) {
	for written := 0; written < len(data); {
		chunk := min(len(data)-written, most)
		if status, _, _ := bcrypt.data.Call(d.object, uintptr(unsafe.Pointer(&data[written])), uintptr(chunk), 0); status != 0 {
			return written, fmt.Errorf("digest: BCryptHashData: %#x", uint32(status))
		}
		written += chunk
	}
	runtime.KeepAlive(d)
	return len(data), nil
}

// Sum finishes a duplicate, so the digest goes on as it was, as hash.Hash's Sum must.
func (d *digest) Sum(into []byte) []byte {
	var duplicate uintptr
	if status, _, _ := bcrypt.duplicate.Call(d.object, uintptr(unsafe.Pointer(&duplicate)), 0, 0, 0); status != 0 {
		panic(fmt.Errorf("digest: BCryptDuplicateHash: %#x", uint32(status)))
	}
	defer destroyed(duplicate)
	sum := make([]byte, d.algorithm.size)
	if status, _, _ := bcrypt.finish.Call(duplicate, uintptr(unsafe.Pointer(&sum[0])), uintptr(len(sum)), 0); status != 0 {
		panic(fmt.Errorf("digest: BCryptFinishHash: %#x", uint32(status)))
	}
	runtime.KeepAlive(d)
	return append(into, sum...)
}

func (d *digest) Size() int      { return d.algorithm.size }
func (d *digest) BlockSize() int { return d.algorithm.block }
