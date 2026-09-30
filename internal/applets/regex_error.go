package applets

import (
	"errors"
	"fmt"
	"regexp"
	"regexp/syntax"
)

// A pattern regcomp refuses is refused here too, and in busybox's words: xregcomp's `bad regex
// 'PATTERN': REASON`, the same for grep, sed and find, where REASON is the C library's regerror
// text as busybox-w32 prints it, measured. An unmatched `[` was a literal and `a\{` a brace, so
// `grep '['` matched a bracket where busybox refuses the pattern with status 2, and the rest came
// out in Go's words: `error parsing regexp: missing closing ): (?:(a)`.

// badRegex is one refused pattern.
type badRegex struct{ pattern, reason string }

func (e badRegex) Error() string { return fmt.Sprintf("bad regex '%s': %s", e.pattern, e.reason) }

// The reasons, as busybox-w32's regerror words them.
const (
	regexInvalid      = "Invalid regular expression"
	regexBracket      = "Unmatched [ or [^"
	regexParen        = `Unmatched ( or \(`
	regexBrace        = `Unmatched \{`
	regexBraceContent = `Invalid content of \{\}`
	regexRange        = "Invalid range end"
	regexPreceding    = "Invalid preceding regular expression"
	regexBackslash    = "Trailing backslash"
)

// compileRegex compiles pattern as an applet names it: a basic expression unless extended,
// translated, then wrapped (anchored, folded) before Go compiles it. A refusal is badRegex.
func compileRegex(pattern string, extended bool, wrap func(string) string) (*regexp.Regexp, error) {
	translated := pattern
	if !extended {
		var err error
		if translated, err = translateBasicRegex(pattern); err != nil {
			return nil, asBadRegex(pattern, err)
		}
	}
	compiled, err := regexp.Compile(wrap(translated))
	if err != nil {
		return nil, asBadRegex(pattern, err)
	}
	return compiled, nil
}

// asBadRegex names pattern's refusal: the translator's own reason, or what Go refused worded as
// regerror words it. An error that is neither, a back-reference RE2 has no way to match, is kept.
func asBadRegex(pattern string, err error) error {
	if bad, ok := errors.AsType[badRegex](err); ok {
		bad.pattern = pattern
		return bad
	}
	syntaxErr, ok := errors.AsType[*syntax.Error](err)
	if !ok {
		return err
	}
	reason := regexInvalid
	switch syntaxErr.Code {
	case syntax.ErrMissingParen, syntax.ErrUnexpectedParen:
		reason = regexParen
	case syntax.ErrMissingBracket:
		reason = unmatchedBracketReason(pattern)
	case syntax.ErrInvalidRepeatSize:
		reason = regexBraceContent
	case syntax.ErrInvalidCharRange:
		reason = regexRange
	case syntax.ErrMissingRepeatArgument, syntax.ErrInvalidRepeatOp:
		reason = regexPreceding
	case syntax.ErrTrailingBackslash:
		reason = regexBackslash
	}
	return badRegex{pattern: pattern, reason: reason}
}

// unmatchedBracketReason is regerror's word for a `[` never closed: a pattern that ends in it
// is invalid, and one with more after it has an unmatched bracket.
func unmatchedBracketReason(pattern string) string {
	if len(pattern) > 0 && pattern[len(pattern)-1] == '[' {
		return regexInvalid
	}
	return regexBracket
}
