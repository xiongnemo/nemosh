package oilsspec

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Result is how one shell's run of a case compares with what the case expects of that
// shell. The order is sh_spec.py's: a run's result is the least of its assertions'.
type Result int

const (
	Timeout Result = iota // the run did not finish; never the result of an assertion
	Fail
	Bug
	Bug2
	NotImplemented
	OK
	OK2
	OK3
	OK4
	Pass
)

var resultNames = [...]string{"TIME", "FAIL", "BUG", "BUG-2", "N-I", "ok", "ok-2", "ok-3", "ok-4", "pass"}

func (r Result) String() string {
	if r < 0 || int(r) >= len(resultNames) {
		return "Result(" + strconv.Itoa(int(r)) + ")"
	}
	return resultNames[r]
}

// Matched reports whether the run did what the case records of the shell, right or
// wrong: a pass, or the value of an OK, BUG or N-I line.
func (r Result) Matched() bool { return r > Fail }

// MarshalText writes a result by the name sh_spec.py's tables give it.
func (r Result) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

func (r *Result) UnmarshalText(text []byte) error {
	for i, name := range resultNames {
		if name == string(text) {
			*r = Result(i)
			return nil
		}
	}
	return fmt.Errorf("no result is named %q", text)
}

// Output is what one run of a case printed and returned.
type Output struct {
	Stdout, Stderr string
	Status         int
}

// Expected is what a case holds one shell to, one assertion for each value that applies.
type Expected struct {
	assertions []assertion
}

type assertion struct {
	key       string // stdout, stderr or status
	text      string
	status    int
	qualifier string // empty for the case's default
}

// Expect answers what the case holds the shell named by label to, as sh_spec.py's
// CreateAssertions builds it. For stdout, stderr and status each, a qualified line for the
// shell wins over the case's default, and status is 0 when neither gives one. A label
// names its shell by prefix, as there, so bash-5.3 is held to what bash is.
//
// A -json value is decoded to UTF-8. Python 2 compares the unicode string it decodes to
// with the bytes a shell printed, which differs from that only outside ASCII, and no
// vendored value decodes to anything outside ASCII.
func (c Case) Expect(label string) (Expected, error) {
	shell := label
	for _, family := range []string{"osh", "bash"} {
		if strings.HasPrefix(label, family) {
			shell = family
		}
	}
	var expected Expected
	for _, key := range []string{"stdout", "stderr", "status"} {
		if qualified := c.Shells[shell]; qualified != nil {
			found, err := expected.add(key, qualified.Values, qualified.Qualifier)
			if err != nil {
				return Expected{}, fmt.Errorf("line %d: %w", c.Line, err)
			}
			if found {
				continue
			}
		}
		found, err := expected.add(key, c.Default, "")
		if err != nil {
			return Expected{}, fmt.Errorf("line %d: %w", c.Line, err)
		}
		if !found && key == "status" {
			expected.assertions = append(expected.assertions, assertion{key: "status"})
		}
	}
	return expected, nil
}

// add appends the assertions values gives for key, both a plain one and a -json one when
// both are there, and answers whether there were any.
func (e *Expected) add(key string, values map[string]string, qualifier string) (bool, error) {
	found := false
	if value, given := values[key]; given {
		held := assertion{key: key, text: value, qualifier: qualifier}
		if key == "status" {
			n, err := strconv.Atoi(strings.Trim(value, asciiSpace))
			if err != nil {
				return false, fmt.Errorf("status %q is not a number", value)
			}
			held.text, held.status = "", n
		}
		e.assertions = append(e.assertions, held)
		found = true
	}
	if encoded, given := values[key+"-json"]; given && key != "status" {
		var value string
		if err := json.Unmarshal([]byte(encoded), &value); err != nil {
			return false, fmt.Errorf("%s-json %s is not a JSON string", key, encoded)
		}
		e.assertions = append(e.assertions, assertion{key: key, text: value, qualifier: qualifier})
		found = true
	}
	return found, nil
}

// Check scores a run: Fail if any assertion is not met, and otherwise the least of the
// qualifiers of the values met, Pass when all were defaults. It says what differed.
func (e Expected) Check(run Output) (Result, []string) {
	result := Pass
	var messages []string
	for _, held := range e.assertions {
		if message := held.unmet(run); message != "" {
			result = Fail
			messages = append(messages, message)
			continue
		}
		result = min(result, qualifierResult(held.qualifier))
	}
	// A Python traceback on stderr fails any case in sh_spec.py, since it is how OSH crashes.
	if strings.Contains(run.Stderr, "Traceback (most recent") {
		result = Fail
		messages = append(messages, "stderr: a Python traceback")
	}
	return result, messages
}

// unmet says how a run differs from what the assertion holds it to, or nothing.
func (held assertion) unmet(run Output) string {
	switch held.key {
	case "status":
		if run.Status != held.status {
			return fmt.Sprintf("status: expected %d, got %d", held.status, run.Status)
		}
	case "stdout":
		if run.Stdout != held.text {
			return fmt.Sprintf("stdout: expected %q, got %q", held.text, run.Stdout)
		}
	default:
		if run.Stderr != held.text {
			return fmt.Sprintf("stderr: expected %q, got %q", held.text, run.Stderr)
		}
	}
	return ""
}

// qualifierResult is sh_spec.py's QualifierToResult: a qualifier it does not know, such as
// OK-5, which its grammar admits, counts as a pass.
func qualifierResult(qualifier string) Result {
	switch qualifier {
	case "BUG":
		return Bug
	case "BUG-2":
		return Bug2
	case "N-I":
		return NotImplemented
	case "OK":
		return OK
	case "OK-2":
		return OK2
	case "OK-3":
		return OK3
	case "OK-4":
		return OK4
	}
	return Pass
}
