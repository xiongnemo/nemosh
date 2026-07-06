package runtime

import "strings"

type listSegment struct {
	text     string
	operator string
}

func splitList(line string) []listSegment {
	var segments []listSegment
	for len(line) > 0 {
		andIndex := strings.Index(line, "&&")
		orIndex := strings.Index(line, "||")
		if andIndex < 0 && orIndex < 0 {
			segments = append(segments, listSegment{text: strings.TrimSpace(line)})
			break
		}
		index := andIndex
		operator := "&&"
		if index < 0 || (orIndex >= 0 && orIndex < index) {
			index = orIndex
			operator = "||"
		}
		segments = append(segments, listSegment{text: strings.TrimSpace(line[:index])})
		segments = append(segments, listSegment{operator: operator})
		line = line[index+2:]
	}
	return segments
}
