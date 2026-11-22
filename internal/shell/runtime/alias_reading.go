package runtime

import "fmt"

// The aliases a text is read with in bash mode, and the parses that read it so; see alias_read.go.

// readingAliases is what a text read now has substituted as it is read: in bash mode with
// expand_aliases on, the aliases defined; nil otherwise, nemosh and sh substituting as a command
// runs instead.
func (r Runtime) readingAliases() map[string]string {
	if !r.BashMode() || r.options == nil || !r.options.expandAliases || len(r.aliases) == 0 {
		return nil
	}
	return r.aliases
}

// aliasReading changes whenever readingAliases' answer does: 0 when it is nil, and otherwise a
// hash of every name and value, the same in whatever order the map gives them. A text read with
// one is read again from the next line when it changes; see runChunkLines.
func (r Runtime) aliasReading() uint64 {
	aliases := r.readingAliases()
	if aliases == nil {
		return 0
	}
	sum := uint64(len(aliases)) | 1<<63
	for name, value := range aliases {
		hash := uint64(14695981039346656037)
		for _, text := range [...]string{name, "=", value} {
			for index := 0; index < len(text); index++ {
				hash = (hash ^ uint64(text[index])) * 1099511628211
			}
		}
		sum ^= hash
	}
	return sum
}

// parseReading parses source beginning on line first, as a session's input still to go on when
// session, with aliases substituted as it is read, and in bash mode when bash.
func parseReading(source string, first int, session bool, aliases map[string]string, bash bool) (Script, error) {
	if len(source) > maxParseInputBytes {
		return Script{}, fmt.Errorf("input bytes: %w", errParseLimit)
	}
	return parseScript(source, &parseBudget{numbering: lineNumbering{first: first}, session: session, aliases: aliases, bash: bash}, 0)
}
