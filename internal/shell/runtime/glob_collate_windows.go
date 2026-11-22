package runtime

import (
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// lcMapStringEx is kernel32's LCMapStringEx, which golang.org/x/sys/windows does not wrap, found
// the first time a pattern is sorted in a locale rather than as the shell starts.
var lcMapStringEx struct {
	once sync.Once
	proc *windows.LazyProc
}

// lcmapSortKey asks LCMapStringEx for a sort key.
const lcmapSortKey = 0x00000400

// sortKey is Windows' sort key for text in locale, whose byte order is the order CompareStringEx
// gives with no flags, as Cygwin's wcscoll asks it; one key a name rather than a call for every
// comparison. nil for a locale Windows does not know, or text it cannot take.
func sortKey(locale, text string) []byte {
	name, err := windows.UTF16PtrFromString(locale)
	if err != nil || text == "" {
		return nil
	}
	source, err := windows.UTF16FromString(text)
	if err != nil {
		return nil
	}
	lcMapStringEx.once.Do(func() { lcMapStringEx.proc = windows.NewLazySystemDLL("kernel32.dll").NewProc("LCMapStringEx") })
	procLCMapStringEx := lcMapStringEx.proc
	call := func(key []byte) uintptr {
		var out unsafe.Pointer
		if len(key) > 0 {
			out = unsafe.Pointer(&key[0])
		}
		size, _, _ := procLCMapStringEx.Call(uintptr(unsafe.Pointer(name)), lcmapSortKey,
			uintptr(unsafe.Pointer(&source[0])), uintptr(len(source)-1), uintptr(out), uintptr(len(key)), 0, 0, 0)
		return size
	}
	size := call(nil)
	if size == 0 {
		return nil
	}
	key := make([]byte, size)
	if size = call(key); size == 0 {
		return nil
	}
	return key[:size]
}
