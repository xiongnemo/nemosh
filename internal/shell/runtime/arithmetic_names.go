package runtime

import (
	"context"
	"fmt"
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
	if p.skipping > 0 {
		return 0, nil
	}
	if err := checkArithmeticSubscript(name, p.depth); err != nil {
		return 0, err
	}
	text := strings.TrimSpace(p.valueText(name))
	if text == "" {
		return 0, nil
	}
	if value, ok := parseArithmeticInteger(text); ok {
		return value, nil
	}
	if value, ok := parseArithmeticBase(text); ok {
		return value, nil
	}
	if p.depth >= maxArithmeticNesting || p.nesting >= maxArithmeticNesting {
		return 0, fmt.Errorf("%s: expression recursion level exceeded", name)
	}
	return p.runtime.evaluateArithmeticAt(text, p.depth+1, p.nesting+1)
}

// arithmeticNestingKey carries into an element's subscript how deep the arithmetic it is read
// for already is. A subscript is evaluated as an expression of its own, and it started the
// count afresh, so `x='x[x]'; $(( x ))` went x, x[x], x, x[x] until the stack ran out: a fatal
// error that ends the process, which the fuzzer found on 2026-10-11 in `${x:x}`. bash says
// `expression recursion level exceeded`.
type arithmeticNestingKey struct{}

func withArithmeticNesting(ctx context.Context, nesting int) context.Context {
	return context.WithValue(ctx, arithmeticNestingKey{}, nesting)
}

// arithmeticNestingOf is the nesting ctx carries, 0 for an expression as written.
func arithmeticNestingOf(ctx context.Context) int {
	nesting, _ := ctx.Value(arithmeticNestingKey{}).(int)
	return nesting
}

// once is a name whose indexed subscript has been evaluated to the number it names, so that an
// operator reading the element and writing it back -- `b[i++]++`, `++b[i++]`, `b[i++] += 1`
// -- evaluates the subscript once; see subscriptOnce.
func (p *arithmeticParser) once(name string) (string, error) {
	reference, ok := parseArrayReference(name)
	if !ok || p.skipping > 0 {
		return name, nil
	}
	if err := checkArithmeticSubscript(name, p.depth); err != nil {
		return "", err
	}
	reference, err := p.runtime.subscriptOnce(withArithmeticNesting(context.Background(), p.nesting+1), reference)
	if err != nil {
		return "", err
	}
	return reference.name + "[" + reference.subscript + "]", nil
}

// valueText is the text a name holds: an element for `a[i]`, the name's own value -- a
// computed one included -- otherwise.
func (p *arithmeticParser) valueText(name string) string {
	ctx := withArithmeticNesting(context.Background(), p.nesting+1)
	if reference, ok := parseArrayReference(name); ok {
		elements, _ := p.runtime.elementsFor(ctx, reference)
		return strings.Join(elements, " ")
	}
	value, _ := p.runtime.lookupParameter(ctx, name, 0)
	return value
}
