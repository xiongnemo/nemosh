package runtime

import (
	"fmt"
	"strings"
)

// Namerefs, bash's: `declare -n ref=name` makes ref another name for name. A read of ref
// reads name, a write writes it, and `unset ref` unsets it; `unset -n ref` unsets the nameref
// itself. The value ref holds is the name it leads to, which may be a chain of namerefs or an
// array's element, `a[2]`; a value that is not a name is not followed. busybox has none, and
// `local -n` was "not an option this build has", so a function taking an array by name could
// not be written.

// maxNamerefDepth ends a chain that loops, `declare -n a=b b=a`, as bash's does.
const maxNamerefDepth = 16

// namerefTarget is the name name leads to through namerefs, name itself when it is not one.
// looped is true when the chain goes round.
func (r Runtime) namerefTarget(name string) (target string, looped bool) {
	for range maxNamerefDepth {
		if !r.attributes[name].nameref {
			return name, false
		}
		value, set := r.vars[name]
		if !set || !isNamerefValue(value) {
			return name, false
		}
		name = value
	}
	return name, true
}

// isNamerefValue reports a value a nameref can lead to: a variable's name, or an element of
// an array.
func isNamerefValue(value string) bool {
	if reference, ok := parseArrayReference(value); ok {
		return isValidVariableName(reference.name)
	}
	return isValidVariableName(value)
}

// namerefText rewrites a parameter reference through a nameref: `$ref` and `${ref...}` read
// the target instead, with `#` and `!` in front left as they are. `$ref` leading to an
// element is braced, since `$a[2]` would be a[2]'s text.
func (r Runtime) namerefText(text string) string {
	body, braced := strings.TrimPrefix(text, "$"), false
	if strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}") {
		body, braced = body[1:len(body)-1], true
	}
	prefix := ""
	if braced && len(body) > 1 && (body[0] == '#' || body[0] == '!') {
		prefix, body = body[:1], body[1:]
	}
	end := 0
	for end < len(body) && isNameByte(body[end]) {
		end++
	}
	if end == 0 || body[0] <= '9' && body[0] >= '0' || !r.attributes[body[:end]].nameref {
		return text
	}
	target, looped := r.namerefTarget(body[:end])
	switch {
	// A loop reads as nothing, as bash reads it.
	case looped:
		return ""
	case target == body[:end]:
		return text
	}
	return "${" + prefix + target + body[end:] + "}"
}

// namerefIndirection is `${!ref}` of a nameref, which bash turns round: the name ref leads
// to, where of any other name it is the value of the variable its value names.
func (r Runtime) namerefIndirection(text string) (string, bool) {
	name, ok := strings.CutPrefix(text, "${!")
	name, closed := strings.CutSuffix(name, "}")
	if !ok || !closed || !r.attributes[name].nameref {
		return "", false
	}
	return r.vars[name], true
}

// namerefAssignment is where an assignment to name goes: the target of a nameref, or name.
// A nameref with no value yet takes the value as its target instead, which handled reports.
func (r Runtime) namerefAssignment(name, value string) (target string, handled bool, err error) {
	if !r.attributes[name].nameref {
		return name, false, nil
	}
	if _, set := r.vars[name]; !set {
		if !isNamerefValue(value) {
			return "", true, fmt.Errorf("%s: invalid variable name for name reference", value)
		}
		r.vars[name] = value
		return name, true, nil
	}
	target, looped := r.namerefTarget(name)
	if looped {
		return "", true, fmt.Errorf("%s: circular name reference", name)
	}
	return target, false, nil
}

// namerefElement is name with the array of an element, `ref[1]`, taken through a nameref:
// `local -n a=$1; a[1]=x` writes the caller's array.
func (r Runtime) namerefElement(name string) (string, error) {
	reference, ok := parseArrayReference(name)
	if !ok || !r.attributes[reference.name].nameref {
		return name, nil
	}
	base, err := r.namerefBase(reference.name)
	if err != nil {
		return "", err
	}
	return base + "[" + reference.subscript + "]", nil
}

// namerefBase is the array a subscripted name writes to through a nameref. A nameref that
// leads to an element cannot take a subscript of its own.
func (r Runtime) namerefBase(name string) (string, error) {
	target, looped := r.namerefTarget(name)
	switch {
	case looped:
		return "", fmt.Errorf("%s: circular name reference", name)
	case strings.ContainsRune(target, '['):
		return "", fmt.Errorf("`%s': not a valid identifier", target)
	}
	return target, nil
}

// unsetTargets are the names `unset` removes: through a nameref, what it leads to, its
// element included -- `unset ref` unsets the target, as in bash. A loop is left as it is.
func (r Runtime) unsetTargets(names []string) []string {
	targets := make([]string, 0, len(names))
	for _, name := range names {
		base, subscript, element := splitSubscriptedName(name)
		target, looped := r.namerefTarget(base)
		switch {
		case looped || target == base:
			targets = append(targets, name)
		case element:
			targets = append(targets, target+"["+subscript+"]")
		default:
			targets = append(targets, target)
		}
	}
	return targets
}
