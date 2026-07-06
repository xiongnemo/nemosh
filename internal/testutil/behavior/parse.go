package behavior

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
)

func ParseCase(data []byte) (Case, error) {
	var c Case
	section := ""
	lines := bytes.Split(data, []byte{'\n'})
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(string(lines[i]))
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return c, fmt.Errorf("line %d: expected key = value", i+1)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if strings.HasPrefix(value, "'''") {
			var b strings.Builder
			first := strings.TrimPrefix(value, "'''")
			if strings.HasSuffix(first, "'''") {
				value = strings.TrimSuffix(first, "'''")
			} else {
				if first != "" {
					b.WriteString(first)
					b.WriteByte('\n')
				}
				for i++; i < len(lines); i++ {
					next := string(lines[i])
					if strings.TrimSpace(next) == "'''" {
						break
					}
					b.WriteString(next)
					b.WriteByte('\n')
				}
				value = b.String()
			}
			assignString(&c, section, key, value)
			continue
		}
		if strings.HasPrefix(value, "[") {
			items, err := parseStringArray(value)
			if err != nil {
				return c, fmt.Errorf("line %d: %w", i+1, err)
			}
			assignArray(&c, section, key, items)
			continue
		}
		if section == "expect" && key == "status" {
			status, err := strconv.Atoi(value)
			if err != nil {
				return c, fmt.Errorf("line %d: invalid status: %w", i+1, err)
			}
			c.Expect.Status = status
			continue
		}
		unquoted, err := strconv.Unquote(value)
		if err != nil {
			return c, fmt.Errorf("line %d: invalid string: %w", i+1, err)
		}
		assignString(&c, section, key, unquoted)
	}
	return c, nil
}

func parseStringArray(value string) ([]string, error) {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "[")
	value = strings.TrimSuffix(value, "]")
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		item, err := strconv.Unquote(strings.TrimSpace(part))
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func assignArray(c *Case, section, key string, value []string) {
	if section != "" {
		return
	}
	switch key {
	case "platforms":
		c.Platforms = value
	case "references":
		c.References = value
	case "command":
		c.Command = value
	}
}

func assignString(c *Case, section, key, value string) {
	switch section {
	case "":
		switch key {
		case "id":
			c.ID = value
		case "area":
			c.Area = value
		case "kind":
			c.Kind = value
		case "semantics":
			c.Semantics = value
		case "script":
			c.Script = value
		}
	case "expect":
		switch key {
		case "stdout":
			c.Expect.Stdout = value
		case "stderr":
			c.Expect.Stderr = value
		}
	case "notes":
		switch key {
		case "standard":
			c.Notes.Standard = value
		case "why":
			c.Notes.Why = value
		}
	}
}
