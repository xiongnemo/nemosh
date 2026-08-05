package runtime

func compoundHeader(line string, keyword string) (string, bool) {
	if len(line) <= len(keyword) || line[:len(keyword)] != keyword || !isShellBlank(line[len(keyword)]) {
		return "", false
	}
	return line[len(keyword)+1:], true
}

func isShellBlank(char byte) bool {
	return char == ' ' || char == '\t'
}
