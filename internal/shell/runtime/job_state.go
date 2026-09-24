package runtime

import (
	"context"
	"sort"
	"strings"
	"time"
)

// jobState is what a background job inherits from the shell, in a form that crosses a
// process boundary: exported fields, meant for JSON. It is step one of
// docs/design/background-processes.md, and nothing starts a process with it yet.
//
// The job's program and the functions it can call travel as text, printed from what was
// parsed (script_print.go), and are parsed again on the other side. The environment and
// the working directory are not here: a process is started with both, and the child
// builds its runtime from them the way any nemosh does. What is here is everything else
// clone() copies; job_state_test.go keeps the two from drifting apart.
type jobState struct {
	Program       string                         `json:"program"`
	Line          int                            `json:"line"`
	Functions     string                         `json:"functions"`
	FunctionFiles map[string]string              `json:"functionFiles"`
	Vars          map[string]string              `json:"vars"`
	Indexed       map[string]jobIndexedArray     `json:"indexed"`
	Associative   map[string]jobAssociativeArray `json:"associative"`
	Attributes    map[string]jobAttributes       `json:"attributes"`
	Readonly      []string                       `json:"readonly"`
	Aliases       map[string]string              `json:"aliases"`
	Options       map[string]bool                `json:"options"`
	Invocation    string                         `json:"invocation"`
	Name          string                         `json:"name"`
	Positional    []string                       `json:"positional"`
	Function      string                         `json:"function"`
	Frames        []jobFrame                     `json:"frames"`
	ScriptFile    string                         `json:"scriptFile"`
	Traps         map[string]string              `json:"traps"`
	Umask         uint16                         `json:"umask"`
	DirStack      []string                       `json:"dirStack"`
	// Seconds is what $SECONDS says now, so the child's count carries on from it.
	Seconds           int  `json:"seconds"`
	FunctionDepth     int  `json:"functionDepth"`
	SourceDepth       int  `json:"sourceDepth"`
	ErrExitSuppressed bool `json:"errExitSuppressed"`
	// Descriptors is the job's descriptor table past what the child's standard handles
	// carry, filled by the launcher rather than captured; see job_descriptors.go.
	Descriptors []jobDescriptor `json:"descriptors"`
}

type jobIndexedArray struct {
	Values []string `json:"values"`
	Live   []int    `json:"live"`
}

type jobAssociativeArray struct {
	Keys   []string `json:"keys"`
	Values []string `json:"values"`
}

type jobAttributes struct {
	Integer, Lower, Upper, Exported bool
}

type jobFrame struct {
	Name, File string
	Line       int
}

// captureJobState is the state a job running program would start with.
func (r Runtime) captureJobState(program programNode) jobState {
	var printer scriptPrinter
	printer.statement(program)
	state := jobState{
		Program: printer.out.String(), Line: r.currentLine(),
		FunctionFiles: map[string]string{}, Vars: cloneMap(r.vars), Aliases: cloneMap(r.aliases),
		Indexed: map[string]jobIndexedArray{}, Associative: map[string]jobAssociativeArray{},
		Attributes: map[string]jobAttributes{}, Options: map[string]bool{}, Traps: map[string]string{},
		Invocation: r.options.invocation, Name: r.params.name, Positional: append([]string(nil), r.params.values...),
		Function: r.params.function, ScriptFile: r.scriptFile, Umask: r.mask.value,
		DirStack: append([]string(nil), r.dirStack.below...), FunctionDepth: r.functionDepth,
		SourceDepth: r.sourceDepth, ErrExitSuppressed: r.errExitSuppressed,
	}
	var functions strings.Builder
	for _, name := range r.sortedFunctionNames() {
		definition := r.functions[functionName{value: name}]
		functions.WriteString(printFunction(definition))
		state.FunctionFiles[name] = definition.file
	}
	state.Functions = functions.String()
	for name, values := range r.arrays.values {
		live := []int{}
		for index := range values {
			if r.arrays.isLive(name, index) {
				live = append(live, index)
			}
		}
		state.Indexed[name] = jobIndexedArray{Values: append([]string(nil), values...), Live: live}
	}
	for name, array := range r.arrays.associative {
		entry := jobAssociativeArray{Keys: append([]string(nil), array.order...)}
		for _, key := range array.order {
			entry.Values = append(entry.Values, array.entries[key])
		}
		state.Associative[name] = entry
	}
	for name, attributes := range r.attributes {
		state.Attributes[name] = jobAttributes{attributes.integer, attributes.lower, attributes.upper, attributes.exported}
	}
	for name := range r.readonly {
		state.Readonly = append(state.Readonly, name)
	}
	sort.Strings(state.Readonly)
	for name, flag := range shellOptionFields(r.options) {
		state.Options[name] = *flag
	}
	for name, action := range r.traps {
		state.Traps[string(name)] = action
	}
	for frame := r.frames; frame != nil; frame = frame.outer {
		state.Frames = append(state.Frames, jobFrame{Name: frame.name, File: frame.file, Line: frame.line})
	}
	if seconds, ok := r.dynamicParameter("SECONDS"); ok {
		state.Seconds = atoiOrZero(seconds)
	}
	return state
}

// restoreJobState puts the state on a runtime the child has just made, and answers with the
// job's program, parsed.
func (r *Runtime) restoreJobState(ctx context.Context, state jobState) (Script, error) {
	definitions, err := parseScriptAt(state.Functions, 1)
	if err != nil {
		return Script{}, err
	}
	for _, node := range definitions.program {
		if definition, ok := node.(functionDefinition); ok {
			definition.file = state.FunctionFiles[definition.name.value]
			r.functions[definition.name] = definition
		}
	}
	for name, value := range state.Vars {
		r.vars[name] = value
	}
	for name, array := range state.Indexed {
		r.arrays.values[name] = append([]string(nil), array.Values...)
		live := map[int]bool{}
		for _, index := range array.Live {
			live[index] = true
		}
		r.arrays.present[name] = live
	}
	for name, array := range state.Associative {
		r.arrays.declareAssociative(name)
		for index, key := range array.Keys {
			r.arrays.setKey(name, key, array.Values[index])
		}
	}
	for name, attributes := range state.Attributes {
		r.attributes[name] = variableAttributes{integer: attributes.Integer, lower: attributes.Lower, upper: attributes.Upper, exported: attributes.Exported}
	}
	for _, name := range state.Readonly {
		r.readonly[name] = struct{}{}
	}
	r.aliases = cloneMap(state.Aliases)
	for name, flag := range shellOptionFields(r.options) {
		*flag = state.Options[name]
	}
	r.options.invocation = state.Invocation
	r.params = &parameters{name: state.Name, values: append([]string(nil), state.Positional...), function: state.Function}
	for name, action := range state.Traps {
		r.traps[trapName(name)] = action
	}
	for index := len(state.Frames) - 1; index >= 0; index-- {
		frame := state.Frames[index]
		r.frames = &callFrame{name: frame.Name, file: frame.File, line: frame.Line, outer: r.frames}
	}
	r.scriptFile, r.mask.value = state.ScriptFile, state.Umask
	r.dirStack.below = append([]string(nil), state.DirStack...)
	r.special.started = time.Now().Add(-time.Duration(state.Seconds) * time.Second)
	r.functionDepth, r.sourceDepth, r.errExitSuppressed = state.FunctionDepth, state.SourceDepth, state.ErrExitSuppressed
	return parseScriptAt(state.Program, max(state.Line, 1))
}

// shellOptionFields names every flag in shellOptions, for a codec that has to carry each
// one. A test holds it to the struct: a bool added there and not here fails.
func shellOptionFields(o *shellOptions) map[string]*bool {
	return map[string]*bool{
		"allExport": &o.allExport, "notify": &o.notify, "noClobber": &o.noClobber,
		"errExit": &o.errExit, "noGlob": &o.noGlob, "noExec": &o.noExec, "noUnset": &o.noUnset,
		"verbose": &o.verbose, "xtrace": &o.xtrace, "pipefail": &o.pipefail,
		"errTrace": &o.errTrace, "funcTrace": &o.funcTrace, "noCaseGlob": &o.noCaseGlob,
		"globStar": &o.globStar, "nullGlob": &o.nullGlob, "dotGlob": &o.dotGlob,
		"noCaseMatch": &o.noCaseMatch, "noHiddenGlob": &o.noHiddenGlob,
		"noHidSysGlob": &o.noHidSysGlob, "ignoreEOF": &o.ignoreEOF, "monitor": &o.monitor,
		"vi": &o.vi, "extGlob": &o.extGlob, "autoCD": &o.autoCD,
	}
}

func atoiOrZero(text string) int {
	value := 0
	for _, char := range text {
		if char < '0' || char > '9' {
			return value
		}
		value = value*10 + int(char-'0')
	}
	return value
}
