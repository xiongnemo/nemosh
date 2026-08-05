package runtime

import (
	"errors"
	"fmt"
)

const (
	maxParseInputBytes = 1 << 20
	maxParseDepth      = 64
	maxParseTokens     = 1 << 16
)

var errParseLimit = errors.New("parser resource limit exceeded")

func InputSizeAllowed(size int) bool {
	return size <= maxParseInputBytes
}

type parseBudget struct {
	tokens          int
	heredocs        map[string]pendingHeredoc
	heredocsScanned bool
}

func (budget *parseBudget) heredoc(marker string) (pendingHeredoc, bool) {
	record, ok := budget.heredocs[marker]
	return record, ok
}

func (budget *parseBudget) consumeTokens(count int) error {
	if count > maxParseTokens-budget.tokens {
		return fmt.Errorf("tokens: %w", errParseLimit)
	}
	budget.tokens += count
	return nil
}
