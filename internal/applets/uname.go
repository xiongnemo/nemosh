package applets

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"
)

type unameInfo struct {
	sysname   string
	nodename  string
	release   string
	version   string
	machine   string
	processor string
	platform  string
	os        string
}

type unameApplet struct {
	info func() unameInfo
}

func newUnameApplet() Applet {
	return unameApplet{info: hostUnameInfo}
}

func (unameApplet) Name() string {
	return "uname"
}

func (a unameApplet) Run(ctx context.Context, args []string, _ io.Reader, stdout, _ io.Writer) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	flags, err := parseUnameFlags(args)
	if err != nil {
		return err
	}
	info := a.info()
	fields := make([]string, 0, len(unameFieldOrder))
	for _, field := range unameFieldOrder {
		if flags&field.bit == 0 {
			continue
		}
		value := field.value(info)
		if value != "" {
			fields = append(fields, value)
		}
	}
	_, err = fmt.Fprintln(stdout, strings.Join(fields, " "))
	return err
}

type unameFlag uint16

const (
	unameFlagSysname unameFlag = 1 << iota
	unameFlagNodename
	unameFlagRelease
	unameFlagVersion
	unameFlagMachine
	unameFlagProcessor
	unameFlagPlatform
	unameFlagOS
)

type unameField struct {
	option       byte
	longOptions  []string
	bit          unameFlag
	allSelection bool
	value        func(unameInfo) string
}

var unameFieldOrder = [...]unameField{
	{option: 's', longOptions: []string{"kernel-name"}, bit: unameFlagSysname, allSelection: true, value: func(info unameInfo) string { return info.sysname }},
	{option: 'n', longOptions: []string{"nodename"}, bit: unameFlagNodename, allSelection: true, value: func(info unameInfo) string { return info.nodename }},
	{option: 'r', longOptions: []string{"kernel-release", "release"}, bit: unameFlagRelease, allSelection: true, value: func(info unameInfo) string { return info.release }},
	{option: 'v', longOptions: []string{"kernel-version"}, bit: unameFlagVersion, allSelection: true, value: func(info unameInfo) string { return info.version }},
	{option: 'm', longOptions: []string{"machine"}, bit: unameFlagMachine, allSelection: true, value: func(info unameInfo) string { return info.machine }},
	{option: 'p', longOptions: []string{"processor"}, bit: unameFlagProcessor, value: func(info unameInfo) string { return info.processor }},
	{option: 'i', longOptions: []string{"hardware-platform"}, bit: unameFlagPlatform, value: func(info unameInfo) string { return info.platform }},
	{option: 'o', longOptions: []string{"operating-system"}, bit: unameFlagOS, allSelection: true, value: func(info unameInfo) string { return info.os }},
}

func parseUnameFlags(args []string) (unameFlag, error) {
	if len(args) == 0 {
		return unameFlagSysname, nil
	}
	flags := unameFlag(0)
	for _, arg := range args {
		if strings.HasPrefix(arg, "--") {
			bit, ok := unameFlagForLongOption(arg[2:])
			if !ok {
				return 0, fmt.Errorf("unsupported uname option: %s", arg)
			}
			flags |= bit
			continue
		}
		if len(arg) < 2 || arg[0] != '-' {
			return 0, ErrExitFalse
		}
		if arg == "--" {
			return 0, ErrExitFalse
		}
		for index := 1; index < len(arg); index++ {
			if arg[index] == 'a' {
				flags = allUnameFlags()
				continue
			}
			bit, ok := unameFlagForOption(arg[index])
			if !ok {
				return 0, fmt.Errorf("unsupported uname option: -%c", arg[index])
			}
			flags |= bit
		}
	}
	if flags == 0 {
		return unameFlagSysname, nil
	}
	return flags, nil
}

func unameFlagForOption(option byte) (unameFlag, bool) {
	for _, field := range unameFieldOrder {
		if field.option == option {
			return field.bit, true
		}
	}
	return 0, false
}

func unameFlagForLongOption(option string) (unameFlag, bool) {
	if option == "all" {
		return allUnameFlags(), true
	}
	for _, field := range unameFieldOrder {
		for _, longOption := range field.longOptions {
			if longOption == option {
				return field.bit, true
			}
		}
	}
	return 0, false
}

func allUnameFlags() unameFlag {
	flags := unameFlag(0)
	for _, field := range unameFieldOrder {
		if field.allSelection {
			flags |= field.bit
		}
	}
	return flags
}

// unameUnknown is what a field says when the platform will not answer. busybox
// uses the same word, and inventing a value would be worse than admitting the
// gap -- the rule the rest of this shell follows.
const unameUnknown = "unknown"

func hostUnameInfo() unameInfo {
	nodename, err := os.Hostname()
	if err != nil || nodename == "" {
		nodename = unameUnknown
	}
	machine := unameMachine(runtime.GOARCH)
	release, version := osReleaseAndVersion()
	return unameInfo{
		sysname:   unameSysname(runtime.GOOS),
		nodename:  nodename,
		release:   release,
		version:   version,
		machine:   machine,
		processor: unameUnknown,
		platform:  unameUnknown,
		os:        unameOS(runtime.GOOS),
	}
}

func unameSysname(goos string) string {
	if goos == "windows" {
		return "Windows_NT"
	}
	if goos == "darwin" {
		return "Darwin"
	}
	return "Linux"
}

func unameOS(goos string) string {
	if goos == "windows" {
		return "MS/Windows"
	}
	return "GNU/Linux"
}

func unameMachine(goarch string) string {
	switch goarch {
	case "386":
		return "i686"
	case "amd64":
		return "x86_64"
	case "arm64":
		return "aarch64"
	case "arm":
		return "armv7"
	default:
		return goarch
	}
}
