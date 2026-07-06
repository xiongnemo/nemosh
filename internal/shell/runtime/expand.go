package runtime

import (
	"bytes"
	"context"
	"strconv"
	"strings"
)

func (r Runtime) expandArgs(ctx context.Context, args []string) []string {
	expanded := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "$@" {
			expanded = append(expanded, r.params.values...)
			continue
		}
		expanded = append(expanded, r.expandArg(ctx, arg))
	}
	return expanded
}

func (r Runtime) expandArg(ctx context.Context, arg string) string {
	var b strings.Builder
	for i := 0; i < len(arg); i++ {
		if arg[i] != '$' || i == len(arg)-1 {
			b.WriteByte(arg[i])
			continue
		}
		if arg[i+1] == '(' {
			commandEnd := strings.IndexByte(arg[i+2:], ')')
			if commandEnd < 0 {
				b.WriteByte(arg[i])
				continue
			}
			command := arg[i+2 : i+2+commandEnd]
			b.WriteString(r.commandSubstitution(ctx, command))
			i = i + commandEnd + 2
			continue
		}
		if arg[i+1] == '#' {
			b.WriteString(strconv.Itoa(len(r.params.values)))
			i++
			continue
		}
		if arg[i+1] == '@' {
			b.WriteString(strings.Join(r.params.values, " "))
			i++
			continue
		}
		if '1' <= arg[i+1] && arg[i+1] <= '9' {
			index := int(arg[i+1] - '1')
			if index < len(r.params.values) {
				b.WriteString(r.params.values[index])
			}
			i++
			continue
		}
		nameStart := i + 1
		nameEnd := nameStart
		for nameEnd < len(arg) && isNameByte(arg[nameEnd]) {
			nameEnd++
		}
		if nameEnd == nameStart {
			b.WriteByte(arg[i])
			continue
		}
		b.WriteString(r.vars[arg[nameStart:nameEnd]])
		i = nameEnd - 1
	}
	return b.String()
}

func (r Runtime) commandSubstitution(ctx context.Context, command string) string {
	var stdout bytes.Buffer
	child := Runtime{registry: r.registry, streams: Streams{Stdin: r.streams.Stdin, Stdout: &stdout, Stderr: r.streams.Stderr}, vars: r.vars, traps: map[string]string{}, params: r.params, readonly: r.readonly}
	child.RunScript(ctx, command)
	return strings.TrimRight(stdout.String(), "\n")
}

func isAssignment(arg string) bool {
	name, _, ok := strings.Cut(arg, "=")
	if !ok || name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		if !isNameByte(name[i]) {
			return false
		}
	}
	return true
}

func isNameByte(b byte) bool {
	return ('a' <= b && b <= 'z') || ('A' <= b && b <= 'Z') || ('0' <= b && b <= '9') || b == '_'
}
