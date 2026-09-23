package runtime

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// A name in arithmetic, and what it stands for.
//
// **It read only the stored variables, and only a plain number.** So `$((RANDOM))` was 0
// every time -- RANDOM is computed, not stored -- and so was every other name whose value is
// not a literal: `x=2+3; $((x*2))` was 0 where both references say 10, and `y=z; z=7;
// $((y))` was 0 where both say 7, because in each the value is itself an expression and is
// evaluated in turn. And an element was not a name at all: `$(( a[0] + a[2] ))` was
// `unexpected "["`, which ruled out the counting idiom `(( count[$k]++ ))`.

// maxArithmeticNesting bounds a value that refers to itself, `x=x`, which bash reports as
// recursion rather than following forever.
const maxArithmeticNesting = 32

// isArithmeticName is a token that names a value: a variable, or an element of one.
func isArithmeticName(token string) bool {
	if isVariableName(token) {
		return true
	}
	_, element := parseArrayReference(token)
	return element
}

// lookup is a name's value as arithmetic sees it: empty or unset is zero, a number is that
// number, and anything else is an expression, evaluated.
func (p *arithmeticParser) lookup(name string) (int64, error) {
	text := strings.TrimSpace(p.valueText(name))
	if text == "" {
		return 0, nil
	}
	if value, err := strconv.ParseInt(text, 0, 64); err == nil {
		return value, nil
	}
	if value, ok := parseArithmeticBase(text); ok {
		return value, nil
	}
	if p.depth >= maxArithmeticNesting {
		return 0, fmt.Errorf("%s: expression recursion level exceeded", name)
	}
	return p.runtime.evaluateArithmeticAt(text, p.depth+1)
}

// valueText is the text a name holds: an element for `a[i]`, the name's own value -- a
// computed one included -- otherwise.
func (p *arithmeticParser) valueText(name string) string {
	ctx := context.Background()
	if reference, ok := parseArrayReference(name); ok {
		elements, _ := p.runtime.elementsFor(ctx, reference)
		return strings.Join(elements, " ")
	}
	value, _ := p.runtime.lookupParameter(ctx, name, 0)
	return value
}
