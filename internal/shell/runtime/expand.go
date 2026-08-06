package runtime

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (r Runtime) expandWord(ctx context.Context, item word, savedStatus int) []string {
	fields := []string{""}
	for _, part := range item.parts {
		switch part.kind {
		case wordPartLiteral, wordPartEscaped:
			fields[len(fields)-1] += part.text
		case wordPartParameter:
			values := r.expandParameterPart(part, savedStatus)
			if part.text == "$@" && part.quote != quoteSingle {
				if len(values) == 0 {
					if len(item.parts) == 1 {
						return nil
					}
					continue
				}
				fields[len(fields)-1] += values[0]
				fields = append(fields, values[1:]...)
				continue
			}
			fields[len(fields)-1] += values[0]
		case wordPartCommandSubstitution:
			if part.script != nil {
				fields[len(fields)-1] += r.commandSubstitutionScript(ctx, *part.script)
			}
		}
	}
	if len(item.parts) == 0 && !item.quotedEmpty {
		return nil
	}
	if item.expandTilde && len(fields) > 0 {
		fields[0] = r.expandHomeTilde(fields[0])
	}
	return fields
}

func (r Runtime) expandHomeTilde(value string) string {
	if value != "~" && !strings.HasPrefix(value, "~/") {
		return value
	}
	home := r.vars["HOME"]
	if home == "" {
		home = r.vars["USERPROFILE"]
	}
	if home == "" {
		return value
	}
	if value == "~" {
		return home
	}
	return strings.TrimRight(home, `/\`) + "/" + strings.TrimPrefix(value, "~/")
}

func (r Runtime) expandParameterPart(part wordPart, savedStatus int) []string {
	text := part.text
	switch text {
	case "$0":
		return []string{r.params.name}
	case "$?":
		return []string{strconv.Itoa(savedStatus)}
	case "$#":
		return []string{strconv.Itoa(len(r.params.values))}
	case "$@":
		return append([]string(nil), r.params.values...)
	case "$*":
		return []string{strings.Join(r.params.values, " ")}
	}
	if len(text) == 2 && '1' <= text[1] && text[1] <= '9' {
		index := int(text[1] - '1')
		if index < len(r.params.values) {
			return []string{r.params.values[index]}
		}
		return []string{""}
	}
	if strings.HasPrefix(text, "${") && strings.HasSuffix(text, "}") {
		body := text[2 : len(text)-1]
		if expanded, ok := r.expandDefaultParameter(body, savedStatus); ok {
			return []string{expanded}
		}
		return []string{text}
	}
	return []string{r.vars[strings.TrimPrefix(text, "$")]}
}

func (r Runtime) commandSubstitutionScript(ctx context.Context, script Script) string {
	var stdout bytes.Buffer
	child, err := r.snapshot(ctx)
	if err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return ""
	}
	table := child.fds
	if err := table.bindBorrowedWriter(1, &stdout); err != nil {
		child.jobScope.cancelAndDrain()
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", errors.Join(err, table.closeAll()))
		return ""
	}
	child = child.withFDTable(table)
	child.traps = map[trapName]string{}
	child.executePrepared(ctx, script)
	child.jobScope.cancelAndDrain()
	if err := table.closeAll(); err != nil {
		fmt.Fprintf(r.streams.Stderr, "nemosh: %v\n", err)
		return ""
	}
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
