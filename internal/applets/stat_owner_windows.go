package applets

import (
	"strings"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The owner busybox-w32's stat reports (win32/mingw.c:565-649), and the names it gives an owner.

const (
	// accountAuthority is SECURITY_NT_NON_UNIQUE, the first sub-authority of a local or domain
	// account's SID, S-1-5-21-....
	accountAuthority = 21
	// fullAccess is what busybox-w32 asks that a file another account owns let this one do before
	// it gives others the owner's permissions: FILE_GENERIC_READ, and write, append and execute.
	fullAccess = 0x1200af
	// fileAllAccess is FILE_ALL_ACCESS, which x/sys/windows does not declare.
	fileAllAccess = 0x1f01ff
)

// fileOwner is busybox-w32's file_owner: defaultWindowsUID for a file the account it runs as
// owns, or that nobody does; a local or domain account's relative ID for one such an account
// owns, when that is from 500 up to defaultWindowsUID; and 0, root, for the rest. Of a file
// another account owns, shared is whether this one, unelevated, may read, write and run it,
// in which case busybox gives others the owner's permissions.
func fileOwner(handle windows.Handle) (uid uint32, shared bool) {
	self := processUserSID()
	if self == nil {
		return defaultWindowsUID, false
	}
	descriptor, err := windows.GetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return 0, false
	}
	owner, _, err := descriptor.Owner()
	switch {
	case err != nil || owner == nil:
		return 0, false
	case owner.Equals(self) || owner.String() == "S-1-0-0":
		return defaultWindowsUID, false
	case owner.IdentifierAuthority() == windows.SECURITY_NT_AUTHORITY && owner.SubAuthorityCount() == 5 &&
		owner.SubAuthority(0) == accountAuthority:
		if id := owner.SubAuthority(4); id >= 500 && id < defaultWindowsUID {
			uid = id
		}
	}
	return uid, currentUserID() != 0 && grantsFullAccess(descriptor)
}

// processUserSID is the account this process runs as, asked once.
var processUserSID = sync.OnceValue(func() *windows.SID {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil
	}
	return user.User.Sid
})

// impersonationToken is the token AccessCheck asks about: this process's, duplicated to be
// impersonated, as busybox-w32 duplicates its own. Asked once, and kept.
var impersonationToken = sync.OnceValue(func() windows.Token {
	var process, duplicate windows.Token
	if windows.OpenProcessToken(windows.CurrentProcess(),
		windows.TOKEN_IMPERSONATE|windows.TOKEN_QUERY|windows.TOKEN_DUPLICATE|windows.STANDARD_RIGHTS_READ, &process) != nil {
		return 0
	}
	defer process.Close()
	if windows.DuplicateTokenEx(process, 0, nil, windows.SecurityImpersonation, windows.TokenImpersonation, &duplicate) != nil {
		return 0
	}
	return duplicate
})

var procAccessCheck = windows.NewLazySystemDLL("advapi32.dll").NewProc("AccessCheck")

// genericMapping and privilegeSet are GENERIC_MAPPING and a PRIVILEGE_SET of one, which
// x/sys/windows does not declare, laid out as documented.
type genericMapping struct{ read, write, execute, all uint32 }

type privilegeSet struct {
	count, control uint32
	privilege      windows.LUIDAndAttributes
}

// grantsFullAccess is whether the file's security lets this account do all fullAccess names.
func grantsFullAccess(descriptor *windows.SECURITY_DESCRIPTOR) bool {
	token := impersonationToken()
	if token == 0 {
		return false
	}
	mapping := genericMapping{windows.FILE_GENERIC_READ, windows.FILE_GENERIC_WRITE, windows.FILE_GENERIC_EXECUTE, fileAllAccess}
	var privileges privilegeSet
	length := uint32(unsafe.Sizeof(privileges))
	var granted uint32
	var allowed int32
	result, _, _ := procAccessCheck.Call(uintptr(unsafe.Pointer(descriptor)), uintptr(token), windows.MAXIMUM_ALLOWED,
		uintptr(unsafe.Pointer(&mapping)), uintptr(unsafe.Pointer(&privileges)), uintptr(unsafe.Pointer(&length)),
		uintptr(unsafe.Pointer(&granted)), uintptr(unsafe.Pointer(&allowed)))
	return result != 0 && allowed != 0 && granted&fullAccess == fullAccess
}

// busyboxAccountName is busybox-w32's getpwuid and getgrgid (win32/mingw.c:1313-1349): root for
// 0, the account it runs as for defaultWindowsUID, a blank in that name made an underscore as
// its get_user_name makes one; and UNKNOWN, stat's word for none, for any other.
func busyboxAccountName(id uint32) string {
	switch id {
	case 0:
		return "root"
	case defaultWindowsUID:
		if name := accountName(); name != "" {
			return strings.ReplaceAll(name, " ", "_")
		}
	}
	return "UNKNOWN"
}

// currentStatOwner is the uid, gid and their names of the account this runs as.
func currentStatOwner() (uint32, uint32, string, string) {
	name := busyboxAccountName(defaultWindowsUID)
	return defaultWindowsUID, defaultWindowsUID, name, name
}
