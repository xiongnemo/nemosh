package applets

import (
	"context"
	"fmt"
	"io"
	"os/user"
	"strings"
)

// newIDApplet implements the subset of POSIX `id` that answers "who am I", in
// the shape busybox-w32 uses. Measured there on 2026-08-08:
//
//	uid=4095(nemo) gid=4095(nemo) groups=4095(nemo)
//
// The identity model is busybox-w32's, not POSIX's, because Windows has no uid.
// busybox maps the question a shell actually asks -- am I privileged -- onto
// elevation: getuid returns 0 only when the process is elevated *and* the
// Administrators group is enabled in its token, and DEFAULT_UID (4095)
// otherwise (win32/mingw.c:1292, include/mingw.h:22). That is what makes
// `[ "$(id -u)" = 0 ]` in a prompt mean something here.
func newIDApplet() Applet {
	return simpleApplet{name: "id", runContext: func(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
		options, operands, err := parseAppletOptions(args, "ugGn", "")
		if err != nil {
			return err
		}
		if len(operands) > 0 {
			return fmt.Errorf("%s: this build reports only the current user", operands[0])
		}

		identity := currentIdentity()
		wantName := options.has('n')
		switch {
		case options.has('u'):
			return writeIDField(stdout, wantName, identity.uid, identity.user)
		case options.has('g'):
			return writeIDField(stdout, wantName, identity.gid, identity.group)
		case options.has('G'):
			return writeIDField(stdout, wantName, identity.gid, identity.group)
		}
		if wantName {
			return fmt.Errorf("-n: needs one of -u, -g or -G to say which name")
		}
		_, err = fmt.Fprintf(stdout, "uid=%d(%s) gid=%d(%s) groups=%d(%s)\n",
			identity.uid, identity.user, identity.gid, identity.group, identity.gid, identity.group)
		return err
	}}
}

func writeIDField(stdout io.Writer, wantName bool, number int, name string) error {
	if wantName {
		_, err := fmt.Fprintln(stdout, name)
		return err
	}
	_, err := fmt.Fprintln(stdout, number)
	return err
}

type processIdentity struct {
	uid   int
	gid   int
	user  string
	group string
}

// rootUserName is what uid 0 is called.
//
// The name follows the number, which is busybox's model rather than an
// invention of ours: getpwuid(0) answers "root" and getpwuid(DEFAULT_UID)
// answers the Windows account name (win32/mingw.c:1313-1320). Windows has no
// root account, so the name is exactly as synthetic as the number -- and it has
// to be, because a prompt reading `nemo` while `id -u` reads 0 tells the reader
// two different things about one process. Under gsudo that is what happened.
const rootUserName = "root"

// currentIdentity answers with what the platform can say, mapped through the
// same rule `id -u` uses.
func currentIdentity() processIdentity {
	name := CurrentUserName()
	if name == "" {
		name = "unknown"
	}
	uid := currentUserID()
	return processIdentity{uid: uid, gid: uid, user: name, group: name}
}

// CurrentUserName is the name belonging to this process's identity, or empty
// when the platform cannot say.
//
// Exported because the prompt has to agree with `id`. bash and busybox both take
// `\u` from the passwd entry for the effective uid rather than from $USER, which
// is why an elevated busybox says root while USERNAME still says nemo.
func CurrentUserName() string {
	return identityName(currentUserID(), accountName())
}

// identityName is the mapping on its own, so it can be checked without being
// elevated -- which is the only part of this a test could not otherwise reach.
func identityName(uid int, account string) string {
	if uid == 0 {
		return rootUserName
	}
	return account
}

// accountName is what the operating system calls the logged-in user, empty when
// it will not say.
func accountName() string {
	current, err := user.Current()
	if err != nil || current.Username == "" {
		return ""
	}
	// Windows reports DOMAIN\user; the bare account name is what a prompt and a
	// comparison both want.
	if _, bare, found := strings.Cut(current.Username, `\`); found {
		return bare
	}
	return current.Username
}
