package applets

import (
	_ "embed"
	"fmt"
)

// bcMathLibrary is what `bc -l` loads first: e, l, s, c, a and j, written in bc itself.
//
// It is gavinhoward/bc's gen/lib.bc, unmodified, with its notice (THIRD-PARTY-NOTICES.md).
// busybox's bc is a port of that project, so the two run the same series on the same exact
// arithmetic and answer alike to the last digit -- which is what makes a maths library safe
// to have: a series written afresh would be wrong in the last digits of an answer that still
// looked right. GNU's libmath.b is GPL, and was not a choice.
//
//go:embed bc_lib.bc
var bcMathLibrary string

// loadMathLibrary defines the library's functions and sets scale to 20, as `-l` does in both
// references.
func (in *bcInterp) loadMathLibrary() error {
	if bad := in.runSource(bcMathLibrary); bad {
		return fmt.Errorf("the maths library did not load")
	}
	in.variables["scale"] = decimalFromInt(20)
	return nil
}
