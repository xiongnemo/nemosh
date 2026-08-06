package runtime

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var errComSpecMissing = errors.New("COMSPEC is not set")

var errBatchOperandUnsupported = errors.New("unsupported batch operand")

// hasBatchSuffix mirrors busybox-w32 has_bat_suffix: the last dot in the base
// name, at most three characters of suffix, compared case-insensitively.
func hasBatchSuffix(name string) bool {
	return hasWindowsSuffix(name, windowsExecutableSuffixes[3:])
}

func hasWindowsSuffix(name string, suffixes []string) bool {
	base := name
	if index := strings.LastIndexAny(name, `/\`); index >= 0 {
		base = name[index+1:]
	}
	dot := strings.LastIndex(base, ".")
	if dot < 0 || len(base)-dot > 4 {
		return false
	}
	for _, suffix := range suffixes {
		if strings.EqualFold(base[dot:], suffix) {
			return true
		}
	}
	return false
}

func comSpecPath(vars map[string]string) (string, error) {
	if comspec := vars["COMSPEC"]; comspec != "" {
		return comspec, nil
	}
	if systemRoot := vars["SYSTEMROOT"]; systemRoot != "" {
		return strings.TrimRight(systemRoot, `/\`) + `\System32\cmd.exe`, nil
	}
	return "", errComSpecMissing
}

// comSpecCommandLine builds the raw command line for `cmd.exe /d /s /c`. /d
// suppresses AutoRun and /s makes cmd strip exactly the outer quote pair and
// take the remainder verbatim, so the whole inner command is wrapped once.
func comSpecCommandLine(comspec, script string, args []string) (string, error) {
	inner, err := quoteBatchOperand(script)
	if err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(`"` + comspec + `" /d /s /c "` + inner)
	for _, arg := range args {
		quoted, quoteErr := quoteBatchOperand(arg)
		if quoteErr != nil {
			return "", quoteErr
		}
		builder.WriteString(" " + quoted)
	}
	builder.WriteString(`"`)
	return builder.String(), nil
}

// quoteBatchOperand quotes one operand for cmd. Wrapping in quotes and doubling
// an embedded quote is cmd's own convention, and it delivers metacharacters such
// as & | > to the script whole. Two operands are refused instead: a line break
// cannot be carried on a command line at all, and a second percent sign closes a
// variable reference that cmd expands before the script ever starts. Whether the
// name between them is defined is not knowable from the operand, so the second
// percent is the tripwire; a lone percent is always literal and passes through.
func quoteBatchOperand(operand string) (string, error) {
	if strings.ContainsAny(operand, "\r\n") {
		return "", fmt.Errorf("cmd.exe cannot carry the line break in %q: %w", operand, errBatchOperandUnsupported)
	}
	if strings.Count(operand, "%") > 1 {
		return "", fmt.Errorf("cmd.exe would expand the variable reference in %q: %w", operand, errBatchOperandUnsupported)
	}
	return `"` + strings.ReplaceAll(operand, `"`, `""`) + `"`, nil
}

// externalCommand builds the child process for an already-resolved executable.
// Windows launches a batch file through the command processor on its own, but it
// does so with the command line Go composed under CommandLineToArgvW rules,
// which cmd does not share: an unquoted & splits the line and \" arrives
// literally. Crossing the boundary explicitly is the only way to control that
// quoting, and it is what docs/design/windows-execution-model.md already asks for.
func (r Runtime) externalCommand(ctx context.Context, executable string, args []string) (*exec.Cmd, error) {
	if runtime.GOOS != "windows" || !hasBatchSuffix(executable) {
		return exec.CommandContext(ctx, executable, args...), nil
	}
	comspec, err := comSpecPath(r.vars)
	if err != nil {
		return nil, err
	}
	line, err := comSpecCommandLine(comspec, executable, args)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, comspec)
	applyRawCommandLine(command, line)
	return command, nil
}
