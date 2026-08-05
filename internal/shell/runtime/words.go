package runtime

func splitWords(line string) ([]string, error) {
	tokens, err := scanShellTokens(line)
	if err != nil {
		return nil, err
	}
	return tokenValues(tokens), nil
}
