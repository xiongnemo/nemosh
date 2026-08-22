package applets

import (
	"context"
	"io"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type Applet interface {
	Name() string
	Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error
}

type Registry struct {
	applets map[string]Applet
}

func NewRegistry(items ...Applet) Registry {
	r := Registry{applets: make(map[string]Applet, len(items))}
	for _, item := range items {
		r.applets[item.Name()] = contextApplet{Applet: item}
	}
	return r
}

type contextApplet struct {
	Applet
}

func (a contextApplet) Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	// Before the applet's own parser, which is the whole point: each of them
	// rejected `--help` in its own words, and this is the one place all of them
	// pass through. See help_option.go for which applets are excluded and why.
	if helpRequested(a.Applet.Name(), args) && writeUsage(a.Applet.Name(), stdout) {
		return nil
	}
	return a.Applet.Run(ctx, args, contextReader{ctx: ctx, reader: stdin}, stdout, stderr)
}

func (r Registry) Lookup(name string) (Applet, bool) {
	applet, ok := r.applets[name]
	return applet, ok
}

// Names lists every registered applet, sorted, as a fresh slice. It backs
// `nemosh --list`, whose output generates Scoop shims, so the order has to be
// stable across runs rather than a map's.
func (r Registry) Names() []string {
	names := make([]string, 0, len(r.applets))
	for name := range r.applets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func InvocationName(args []string) string {
	if len(args) == 0 || args[0] == "" {
		return ""
	}
	base := filepath.Base(args[0])
	if runtime.GOOS == "windows" && strings.EqualFold(filepath.Ext(base), ".exe") {
		base = base[:len(base)-len(".exe")]
	}
	return strings.ToLower(base)
}

// DefaultRegistry is every applet this build carries, portable ones first and
// then whatever the platform adds.
//
// The list varies by platform on purpose, and only downwards: a name is added
// where it means something and left alone where it does not. `su` is registered
// on Windows and nowhere else, because Unix already has a real `su` in
// util-linux and an applet of that name would shadow it -- ours does no setuid,
// reads no user database, and answers only to `root`. busybox-w32 makes the same
// split by building suw32 under PLATFORM_MINGW32 alone.
var DefaultRegistry = NewRegistry(append(portableApplets(), platformApplets()...)...)

func portableApplets() []Applet {
	return []Applet{
		newTrueApplet(),
		newFalseApplet(),
		newYesApplet(),
		newEchoApplet(),
		newPrintfApplet(),
		newCatApplet(),
		newPwdApplet(),
		newHeadApplet(),
		newTailApplet(),
		newWcApplet(),
		newDateApplet(),
		newSortApplet(),
		newUniqApplet(),
		newCutApplet(),
		newEnvApplet(),
		newPrintenvApplet(),
		newTestApplet("test"),
		newTestApplet("["),
		newBasenameApplet(),
		newDirnameApplet(),
		newWinpathApplet(),
		newPosixpathApplet(),
		newTouchApplet(),
		newRmApplet(),
		newMkdirApplet(),
		newRmdirApplet(),
		newLsApplet(),
		newLnApplet(),
		newReadlinkApplet(),
		newRealpathApplet(),
		newSleepApplet(),
		newUnameApplet(),
		newIDApplet(),
		newPgrepApplet(),
		newPkillApplet(),
		newTrApplet(),
		newTeeApplet(),
		newSeqApplet(),
		newClearApplet(),
		newWhoamiApplet(),
		newMktempApplet(),
		newCpApplet(),
		newMvApplet(),
		newChmodApplet(),
		newGrepApplet(),
		newSedApplet(),
		newFindApplet(),
		newXargsApplet(),
		newTacApplet(),
		newRevApplet(),
		newNlApplet(),
		newBase64Applet(),
		newSha256sumApplet(),
		newMd5sumApplet(),
		newSha1sumApplet(),
		newSha384sumApplet(),
		newSha512sumApplet(),
		newSha3sumApplet(),
		newCksumApplet(),
		newCrc32Applet(),
		newSumApplet(),
		newFreeApplet(),
		newFactorApplet(),
		newFoldApplet(),
		newTsortApplet(),
		newStringsApplet(),
		newAsciiApplet(),
		newExpandApplet(),
		newUnexpandApplet(),
		newJoinApplet(),
		newBase32Applet(),
		newShufApplet(),
		newDos2unixApplet(),
		newUnix2dosApplet(),
		newIconvApplet(),
		newGzipApplet(),
		newGunzipApplet(),
		newZcatApplet(),
		newBunzip2Applet(),
		newBzcatApplet(),
		newTarApplet(),
		newUnzipApplet(),
		newNanoApplet(),
		newMicroApplet(),
		newOdApplet(),
		newHexdumpApplet(),
		newHdApplet(),
		newUuencodeApplet(),
		newUudecodeApplet(),
		newDiffApplet(),
		newPatchApplet(),
		newCmpApplet(),
		newCommApplet(),
		newPasteApplet(),
		newXxdApplet(),
		newSplitApplet(),
		newPsApplet(),
		newTopApplet(),
		newDuApplet(),
		newStatApplet(),
		newExprApplet(),
	}
}
