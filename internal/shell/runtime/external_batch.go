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

// quoteBatchOperand quotes one operand for cmd, and only when it has to.
//
// Quoting every operand was the defect: `%1` arrived as `"arg1"` rather than
// arg1, so `if "%1"=="foo"` -- the commonest thing a batch file does with its
// arguments -- compared `""foo""` against `"foo"` and never matched. cmd.exe
// itself puts quotes in `%1` only when the caller typed them, and busybox's
// quote_arg holds the same line on the CommandLineToArgvW side of the boundary:
// `int quoted = !*arg;` and then only a space or tab sets it
// (win32/process.c:123-128). Nemosh diverged from both.
//
// What forces quotes here is cmd's parsing rather than argv's: a space or tab
// would split the operand, and & | < > ^ ( ) would be read as command syntax
// before the script saw them. An embedded quote is doubled, which is cmd's
// convention and not argv's \".
//
// Two operands are refused rather than quoted: a line break cannot be carried
// on a command line at all, and a second percent sign closes a variable
// reference that cmd expands before the script starts. Whether the name between
// them is defined is not knowable from the operand, so the second percent is
// the tripwire; a lone percent is always literal and passes through.
func quoteBatchOperand(operand string) (string, error) {
	if strings.ContainsAny(operand, "\r\n") {
		return "", fmt.Errorf("cmd.exe cannot carry the line break in %q: %w", operand, errBatchOperandUnsupported)
	}
	if strings.Count(operand, "%") > 1 {
		return "", fmt.Errorf("cmd.exe would expand the variable reference in %q: %w", operand, errBatchOperandUnsupported)
	}
	if operand != "" && !strings.ContainsAny(operand, cmdQuotingTriggers) {
		return operand, nil
	}
	return `"` + strings.ReplaceAll(operand, `"`, `""`) + `"`, nil
}

const cmdQuotingTriggers = " \t\"&|<>^()"

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
