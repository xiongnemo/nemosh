//go:build windows

package runtime

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// unelevatedToken is this process's token with what makes Windows call it elevated taken out, for
// drop (applets.WithUnelevatedLaunch). It keeps this user, their logon and their groups.
//
// The groups taken out are BUILTIN\Administrators and NT AUTHORITY\Local account and member of
// Administrators group, S-1-5-114, both kept only to deny. All privileges go, but bypassing
// traverse checks, and the integrity level falls to medium. Measured over an sshd session on
// 2026-11-25, whose token is elevated with no linked one to fall back to:
//   - busybox-w32's drop (SAFER's normal-user level, medium integrity) leaves S-1-5-114 enabled,
//     and TokenElevation still reads 1.
//   - Taking out the Administrators group alone left it at 1. Taking out S-1-5-114 too made it 0.
//
// What the token's processes make belongs to this user, under a default DACL that lets this user and
// SYSTEM in. An elevated token's objects belong to the Administrators group, which this token may
// no longer use, so a process run with it could not start one of its own: access denied.
func unelevatedToken() (windows.Token, error) {
	var self windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ALL_ACCESS, &self); err != nil {
		return 0, fmt.Errorf("open this process's token: %w", err)
	}
	defer self.Close()
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return 0, err
	}
	localAdministrators, err := windows.CreateWellKnownSid(windows.WinLocalAccountAndAdministratorSid)
	if err != nil {
		return 0, err
	}
	disabled := []windows.SIDAndAttributes{{Sid: administrators}, {Sid: localAdministrators}}
	// Looked up here, not at every start of the shell: see the init ceiling in product.yml.
	restrict := windows.NewLazySystemDLL("advapi32.dll").NewProc("CreateRestrictedToken")
	if err := restrict.Find(); err != nil {
		return 0, err
	}
	var token windows.Token
	if result, _, err := syscall.SyscallN(restrict.Addr(), uintptr(self), disableMaxPrivilege,
		uintptr(len(disabled)), uintptr(unsafe.Pointer(&disabled[0])), 0, 0, 0, 0, uintptr(unsafe.Pointer(&token))); result == 0 {
		return 0, fmt.Errorf("restrict this process's token: %w", err)
	}
	if err := lowerToMedium(token); err != nil {
		token.Close()
		return 0, err
	}
	if err := ownByUser(token); err != nil {
		token.Close()
		return 0, err
	}
	return token, nil
}

// unelevatedLauncher makes the token and answers with what gives it to a command, which Go then
// starts with CreateProcessAsUser, and what closes it once the command has started. A token that is
// a restricted copy of the caller's own needs no privilege to start a process with.
func unelevatedLauncher() (give func(*exec.Cmd), release func(), err error) {
	token, err := unelevatedToken()
	if err != nil {
		return nil, nil, err
	}
	give = func(command *exec.Cmd) {
		if command.SysProcAttr == nil {
			command.SysProcAttr = &syscall.SysProcAttr{}
		}
		command.SysProcAttr.Token = syscall.Token(token)
	}
	return give, func() { _ = token.Close() }, nil
}

const (
	// disableMaxPrivilege is CreateRestrictedToken's DISABLE_MAX_PRIVILEGE: every privilege but
	// SeChangeNotifyPrivilege goes.
	disableMaxPrivilege = 0x1
	// mediumIntegrity and systemIntegrity are SECURITY_MANDATORY_MEDIUM_RID and _SYSTEM_RID, the
	// last sub-authority of the S-1-16 labels.
	mediumIntegrity = 0x2000
	systemIntegrity = 0x4000
)

// lowerToMedium gives the token medium integrity, where it had more; one that had less keeps it,
// since raising it is not this process's to do.
func lowerToMedium(token windows.Token) error {
	current, err := tokenIntegrityLabel(token)
	if err != nil {
		return fmt.Errorf("read the token's integrity level: %w", err)
	}
	if integrityRID(current.Label.Sid) <= mediumIntegrity {
		return nil
	}
	medium, err := windows.CreateWellKnownSid(windows.WinMediumLabelSid)
	if err != nil {
		return err
	}
	label := windows.Tokenmandatorylabel{Label: windows.SIDAndAttributes{Sid: medium, Attributes: windows.SE_GROUP_INTEGRITY}}
	if err := windows.SetTokenInformation(token, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&label)), label.Size()); err != nil {
		return fmt.Errorf("lower the token to medium integrity: %w", err)
	}
	return nil
}

// tokenIntegrityLabel is the token's mandatory label. It is read into words, not bytes, so the
// pointer at its head is aligned: checkptr refuses a byte buffer taken for the struct under -race,
// as CI's windows-latest job runs it.
func tokenIntegrityLabel(token windows.Token) (*windows.Tokenmandatorylabel, error) {
	buffer := make([]uint64, 16)
	var size uint32
	if err := windows.GetTokenInformation(token, windows.TokenIntegrityLevel, (*byte)(unsafe.Pointer(&buffer[0])), uint32(len(buffer)*8), &size); err != nil {
		return nil, err
	}
	return (*windows.Tokenmandatorylabel)(unsafe.Pointer(&buffer[0])), nil
}

// integrityRID is a mandatory label's level, its last sub-authority, read from its string: the
// accessors that count sub-authorities point into the SID, which checkptr refuses under -race.
func integrityRID(label *windows.SID) uint32 {
	text := label.String()
	rid, err := strconv.ParseUint(text[strings.LastIndexByte(text, '-')+1:], 10, 32)
	if err != nil {
		return systemIntegrity
	}
	return uint32(rid)
}

// ownByUser makes this user the owner of what the token's processes make, and its default DACL
// one that lets this user and SYSTEM in.
func ownByUser(token windows.Token) error {
	user, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("read the token's user: %w", err)
	}
	owner := struct{ Owner *windows.SID }{user.User.Sid}
	if err := windows.SetTokenInformation(token, windows.TokenOwner, (*byte)(unsafe.Pointer(&owner)), uint32(unsafe.Sizeof(owner))); err != nil {
		return fmt.Errorf("make the user the token's owner: %w", err)
	}
	descriptor, err := windows.SecurityDescriptorFromString("D:(A;;GA;;;" + user.User.Sid.String() + ")(A;;GA;;;SY)")
	if err != nil {
		return err
	}
	dacl, _, err := descriptor.DACL()
	if err != nil {
		return err
	}
	defaultDACL := struct{ DefaultDacl *windows.ACL }{dacl}
	if err := windows.SetTokenInformation(token, windows.TokenDefaultDacl, (*byte)(unsafe.Pointer(&defaultDACL)), uint32(unsafe.Sizeof(defaultDACL))); err != nil {
		return fmt.Errorf("set the token's default DACL: %w", err)
	}
	return nil
}
