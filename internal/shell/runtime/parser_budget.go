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
	// numbering is where the lines being parsed came from, for $LINENO; see line_numbers.go.
	numbering lineNumbering
	// session is input a session is still reading, whose last line a `\` continues onto the
	// next it reads; anything else ends where its text does. See numberedLogicalLines.
	session bool
	// warnings are what the parse has to say of a text it accepts, the heredoc the end of a
	// script closed; see collectHeredocs.
	warnings []string
	// aliases are substituted as the text is read, in bash mode; nil otherwise. See
	// alias_read.go.
	aliases map[string]string
	// bash is whether the text is read in bash mode, which takes a few forms busybox refuses as
	// it reads them -- a loop variable that is no name, a `$((` that is no arithmetic -- and
	// refuses them as bash does, when it runs. The zero value is nemosh's, so a budget made
	// anywhere without it refuses what it always did; nested parses share their text's budget,
	// and with it the mode. See parser_bash.go.
	bash bool
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
