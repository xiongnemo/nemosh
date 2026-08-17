package completionspec

import (
	"fmt"
	"strings"
)

// Mapping a parsed document onto Spec.
//
// Written out key by key rather than driven by reflection. Three things come of
// that. The struct tags stay honest, because the differential test decodes the
// same structs with a real TOML implementation through them. `reflect` stays out
// of the binary. And the "keys this format does not have" diagnostic is possible
// at all: a reflective decoder knows only that a key went unused, while this one
// knows what the field names were and can be asked to list them.

// tableReader pulls typed values out of one table, remembering what it was asked
// for and the first thing that was the wrong shape.
//
// The asked-for set is how an unknown key is found: the decoder names every field
// it has, and whatever is left in the file is a key the format does not have.
// That is the same question toml.MetaData.Undecoded answered, and it is why a
// misspelling is refused rather than shrugged at.
type tableReader struct {
	table *tomlTable
	where string
	asked map[string]bool
	err   error
}

func newTableReader(table *tomlTable, where string) *tableReader {
	return &tableReader{table: table, where: where, asked: map[string]bool{}}
}

func (t *tableReader) text(key string) string {
	t.asked[key] = true
	if t.table == nil {
		return ""
	}
	value, present := t.table.values[key]
	if !present {
		return ""
	}
	if value.isList {
		t.fail(key, "a string", "a list")
		return ""
	}
	return value.text
}

func (t *tableReader) list(key string) []string {
	t.asked[key] = true
	if t.table == nil {
		return nil
	}
	value, present := t.table.values[key]
	if !present {
		return nil
	}
	if !value.isList {
		t.fail(key, "a list of strings", "a string")
		return nil
	}
	return value.list
}

func (t *tableReader) fail(key, want, got string) {
	if t.err != nil {
		return
	}
	t.err = fmt.Errorf("%s.%s is %s, and this format wants %s", t.where, key, got, want)
}

// unknown is every key in the table the decoder never asked for.
func (t *tableReader) unknown() []string {
	if t.table == nil {
		return nil
	}
	var rest []string
	for _, key := range t.table.keys() {
		if !t.asked[key] {
			rest = append(rest, t.where+"."+key)
		}
	}
	return rest
}

// decodeDocument reads a spec out of a parsed file.
//
// Every unknown key is collected before anything is reported, so a file with
// three misspellings is fixed in one pass rather than three.
func decodeDocument(document *tomlDocument) (Spec, error) {
	var spec Spec
	var unknown []string
	for _, name := range document.order {
		switch name {
		case "meta":
			meta, rest, err := decodeMeta(document.tables[name])
			if err != nil {
				return Spec{}, err
			}
			spec.Meta, unknown = meta, append(unknown, rest...)
		case "command":
			surface, rest, err := decodeSurface(document.tables[name], "command")
			if err != nil {
				return Spec{}, err
			}
			spec.Command, unknown = surface, append(unknown, rest...)
		case "subcommand":
			for _, table := range document.arrays[name] {
				surface, rest, err := decodeSurface(table, "subcommand")
				if err != nil {
					return Spec{}, err
				}
				// The two are the same shape, which SurfaceFor already relies on;
				// the conversion is what makes the compiler hold them to it.
				spec.Subcommand = append(spec.Subcommand, Subcommand(surface))
				unknown = append(unknown, rest...)
			}
		default:
			unknown = append(unknown, name)
		}
	}
	if len(unknown) > 0 {
		return Spec{}, fmt.Errorf("keys this format does not have: %s", strings.Join(unknown, ", "))
	}
	return spec, nil
}

func decodeMeta(table *tomlTable) (Meta, []string, error) {
	reader := newTableReader(table, "meta")
	meta := Meta{
		DerivedFrom: reader.text("derived-from"),
		ToolVersion: reader.text("tool-version"),
		MeasuredOn:  reader.text("measured-on"),
		GeneratedBy: reader.text("generated-by"),
	}
	return meta, reader.unknown(), reader.err
}

// decodeSurface reads a command or a subcommand, which are one shape: a
// subcommand is a surface like any other, and the fields say so.
func decodeSurface(table *tomlTable, where string) (Command, []string, error) {
	reader := newTableReader(table, where)
	surface := Command{
		Name:       reader.text("name"),
		Operand:    OperandKind(reader.text("operand")),
		Short:      reader.text("short"),
		ValueShort: reader.text("value-short"),
		FileShort:  reader.text("file-short"),
		Long:       reader.list("long"),
		ValueLong:  reader.list("value-long"),
		FileLong:   reader.list("file-long"),
	}
	return surface, reader.unknown(), reader.err
}
