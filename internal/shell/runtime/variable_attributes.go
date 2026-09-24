package runtime

import (
	"context"
	"strconv"
	"strings"
)

// Variable attributes, `declare -i -l -u`, and the `+=` that means something different under
// one of them.
//
// `declare -i n; n=2+3` is 5 in bash: an integer name evaluates what it is given. `-l` and
// `-u` fold case on every assignment. All three were refused -- `not an option this build
// has` -- which at least said so. `x+=y` did not: it ran a command called `x+=y`, reported it
// not found, and left x as it was, so a script building a string or a PATH got the old value
// and carried on. busybox has neither; bash is the reference.
//
// They live beside the variables rather than in them, and assignVar applies them -- the one
// write path -- so a value arriving by `read`, a loop, `export`, arithmetic or an element
// assignment is treated the same as one written with `=`.

// variableAttributes are what `declare` can say about a name beyond its value.
type variableAttributes struct {
	integer bool
	lower   bool
	upper   bool
}

// attributesOf answers for a name or for the array an element belongs to.
func (r Runtime) attributesOf(name string) variableAttributes {
	if reference, ok := parseArrayReference(name); ok {
		name = reference.name
	}
	return r.attributes[name]
}

// applyAttributes is the value an assignment stores: evaluated for an integer name, folded
// for a lower- or upper-case one.
func (r Runtime) applyAttributes(name, value string) (string, error) {
	attributes := r.attributesOf(name)
	if attributes.integer {
		number, err := r.evaluateArithmetic(value)
		if err != nil {
			return "", err
		}
		value = strconv.FormatInt(number, 10)
	}
	switch {
	case attributes.lower:
		value = strings.ToLower(value)
	case attributes.upper:
		value = strings.ToUpper(value)
	}
	return value, nil
}

// applyDeclaredAttributes records what a declare said about a name. `-l` and `-u` replace
// each other, as in bash, and `+i +l +u` take one away.
func (r Runtime) applyDeclaredAttributes(name string, options declareOptions) {
	attributes := r.attributes[name]
	if options.integer {
		attributes.integer = true
	}
	if options.lower || options.upper {
		attributes.lower, attributes.upper = options.lower, options.upper
	}
	for _, letter := range options.removed {
		switch letter {
		case 'i':
			attributes.integer = false
		case 'l':
			attributes.lower = false
		case 'u':
			attributes.upper = false
		}
	}
	if attributes == (variableAttributes{}) {
		delete(r.attributes, name)
		return
	}
	r.attributes[name] = attributes
}

// declareFlags are a name's attributes in the order `declare -p` prints them, which is
// bash's table order: a A i r x l u. "-" when there are none, making `declare --`.
func (r Runtime) declareFlags(name string) string {
	var flags strings.Builder
	switch {
	case r.arrays.isAssociative(name):
		flags.WriteByte('A')
	case r.hasIndexedArray(name):
		flags.WriteByte('a')
	}
	attributes := r.attributes[name]
	if attributes.integer {
		flags.WriteByte('i')
	}
	if r.isReadonly(name) {
		flags.WriteByte('r')
	}
	if _, exported := r.env.LookupEnv(name); exported {
		flags.WriteByte('x')
	}
	if attributes.lower {
		flags.WriteByte('l')
	}
	if attributes.upper {
		flags.WriteByte('u')
	}
	if flags.Len() == 0 {
		return "-"
	}
	return flags.String()
}

// splitAssignmentTarget reads the left side of `name=value` or `name+=value`, reporting the
// name and whether the value is appended.
func splitAssignmentTarget(target string) (string, bool) {
	if name, appended := strings.CutSuffix(target, "+"); appended && name != "" {
		return name, true
	}
	return target, false
}

// appendedValue is what `name+=value` assigns: the old text and the new for a string, and
// the sum for an integer name -- `declare -i n=5; n+=3` is 8, where a string would be 53.
func (r Runtime) appendedValue(name, value string) string {
	ctx := context.Background()
	current, _ := r.lookupParameter(ctx, name, 0)
	if reference, ok := parseArrayReference(name); ok {
		elements, _ := r.elementsFor(ctx, reference)
		current = strings.Join(elements, " ")
	}
	if r.attributesOf(name).integer {
		if strings.TrimSpace(current) == "" {
			current = "0"
		}
		return current + "+(" + value + ")"
	}
	return current + value
}
