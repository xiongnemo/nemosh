package runtime

import (
	"context"
	"strings"
)

func (r Runtime) expandHeredocBody(ctx context.Context, body string, savedStatus int) string {
	var expanded strings.Builder
	for index := 0; index < len(body); {
		if body[index] == '\\' && index+1 < len(body) && body[index+1] == '\n' {
			index += 2
			continue
		}
		if body[index] == '\\' && index+1 < len(body) && (body[index+1] == '$' || body[index+1] == '\\') {
			expanded.WriteByte(body[index+1])
			index += 2
			continue
		}
		if body[index] != '$' {
			expanded.WriteByte(body[index])
			index++
			continue
		}
		if index+1 < len(body) && body[index+1] == '(' {
			end, ok := commandSubstitutionEnd(body, index+2)
			if ok {
				script, err := ParseScript(body[index+2 : end])
				if err == nil {
					expanded.WriteString(r.commandSubstitutionScript(ctx, script, savedStatus))
					index = end + 1
					continue
				}
			}
		}
		end := parameterEnd(body, index+1)
		if end > index+1 {
			values := r.expandParameterPart(ctx, wordPart{kind: wordPartParameter, text: body[index:end]}, savedStatus)
			expanded.WriteString(strings.Join(values, " "))
			index = end
			continue
		}
		expanded.WriteByte(body[index])
		index++
	}
	return expanded.String()
}
