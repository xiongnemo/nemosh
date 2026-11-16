package applets

import (
	"errors"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The Windows calls iconv converts with, as busybox-w32's does (miscutils/iconv.c, which is
// win_iconv): kernel32's code pages for nearly everything, and MLang's for EUC-JP and the
// ISO-2022-JP family, whose kernel32 conversions differ between Windows versions. Looked up on
// first use, so a run that converts nothing loads nothing.

const (
	mbErrInvalidChars    = 0x8
	wcNoBestFitChars     = 0x400
	errInsufficientBuffr = windows.Errno(122)
	iconvCharMax         = 16 // win_iconv's MB_CHAR_MAX: room for one character either way
)

type iconvProcs struct {
	once                                                        sync.Once
	wideToMulti, validCodePage, codePageInfo, mlangIn, mlangOut *windows.LazyProc
	mlangLoaded                                                 bool
}

var iconvCalls iconvProcs

func iconvAPI() *iconvProcs {
	iconvCalls.once.Do(func() {
		kernel := windows.NewLazySystemDLL("kernel32.dll")
		iconvCalls.wideToMulti = kernel.NewProc("WideCharToMultiByte")
		iconvCalls.validCodePage = kernel.NewProc("IsValidCodePage")
		iconvCalls.codePageInfo = kernel.NewProc("GetCPInfo")
		mlang := windows.NewLazySystemDLL("mlang.dll")
		iconvCalls.mlangIn = mlang.NewProc("ConvertINetMultiByteToUnicode")
		iconvCalls.mlangOut = mlang.NewProc("ConvertINetUnicodeToMultiByte")
		iconvCalls.mlangLoaded = iconvCalls.mlangIn.Find() == nil && iconvCalls.mlangOut.Find() == nil
	})
	return &iconvCalls
}

// codePageMaxCharSize is GetCPInfo's MaxCharSize for a code page Windows has, or 0, and its
// lead bytes marked in leads when there is somewhere to mark them.
func codePageMaxCharSize(codePage int, leads *[256]bool) int {
	api := iconvAPI()
	if valid, _, _ := api.validCodePage.Call(uintptr(codePage)); valid == 0 {
		return 0
	}
	var info struct {
		maxCharSize uint32
		defaultChar [2]byte
		leadByte    [12]byte
	}
	if ok, _, _ := api.codePageInfo.Call(uintptr(codePage), uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 0
	}
	for pair := 0; leads != nil && pair+1 < len(info.leadByte) && info.leadByte[pair] != 0; pair += 2 {
		for b := int(info.leadByte[pair]); b <= int(info.leadByte[pair+1]); b++ {
			leads[b] = true
		}
	}
	return int(info.maxCharSize)
}

// multiToWide is MultiByteToWideChar of in into wide, how many it wrote, 0 for an error.
func multiToWide(codePage int, flags uint32, in []byte, wide []uint16) int {
	written, _ := windows.MultiByteToWideChar(uint32(codePage), flags, &in[0], int32(len(in)), &wide[0], int32(len(wide)))
	return int(written)
}

// wideToMulti is WideCharToMultiByte of wide into out: how many it wrote, whether the default
// character stood in for one it could not, and whether out was too small.
func wideToMulti(codePage int, flags uint32, wide []uint16, out []byte, askDefault bool) (int, bool, bool) {
	var usedDefault int32
	usedPointer := uintptr(0)
	if askDefault {
		usedPointer = uintptr(unsafe.Pointer(&usedDefault))
	}
	written, _, err := iconvAPI().wideToMulti.Call(uintptr(codePage), uintptr(flags),
		uintptr(unsafe.Pointer(&wide[0])), uintptr(len(wide)), uintptr(unsafe.Pointer(&out[0])), uintptr(len(out)), 0, usedPointer)
	return int(written), usedDefault != 0, written == 0 && errors.Is(err, errInsufficientBuffr)
}

// mlangToWide is ConvertINetMultiByteToUnicode: how much of in it read and how many it wrote,
// or false for an error.
func mlangToWide(mode *uint32, codePage int, in []byte, wide []uint16) (int, int, bool) {
	read, written := int32(len(in)), int32(len(wide))
	result, _, _ := iconvAPI().mlangIn.Call(uintptr(unsafe.Pointer(mode)), uintptr(codePage),
		uintptr(unsafe.Pointer(&in[0])), uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&wide[0])), uintptr(unsafe.Pointer(&written)))
	return int(read), int(written), result == 0
}

// mlangToMulti is ConvertINetUnicodeToMultiByte: how much of wide it read and how many bytes
// it wrote, or false for an error.
func mlangToMulti(mode *uint32, codePage int, wide []uint16, out []byte) (int, int, bool) {
	read, written := int32(len(wide)), int32(len(out))
	result, _, _ := iconvAPI().mlangOut.Call(uintptr(unsafe.Pointer(mode)), uintptr(codePage),
		uintptr(unsafe.Pointer(&wide[0])), uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&out[0])), uintptr(unsafe.Pointer(&written)))
	return int(read), int(written), result == 0
}
