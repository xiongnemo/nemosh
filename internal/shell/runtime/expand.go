package runtime

import "strings"

func (r Runtime) expandArgs(args []string) []string {
	expanded := make([]string, 0, len(args))
	for _, arg := range args {
		expanded = append(expanded, r.expandArg(arg))
	}
	return expanded
}

func (r Runtime) expandArg(arg string) string {
	var b strings.Builder
	for i := 0; i < len(arg); i++ {
		if arg[i] != '$' || i == len(arg)-1 {
			b.WriteByte(arg[i])
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
