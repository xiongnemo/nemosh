// Package oilsspec measures nemosh with the Oils spec suite vendored in tests/oils. The
// suite is an instrument rather than part of the corpus: its cases run as upstream wrote
// them and pin nothing, as docs/design/reference-methodology.md explains.
package oilsspec

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Upstream is tests/oils/upstream.json, which scripts/oils-vendor.sh writes: the Oils
// commit the copy was taken from, and what each file held there, keyed by its
// slash-separated path in the Oils tree. Skeleton is every name at the top of that tree
// and in its spec/, a directory's ending in a slash, since cases that cd to $REPO_ROOT
// list and glob them.
type Upstream struct {
	Repository string          `json:"repository"`
	Commit     string          `json:"commit"`
	Committed  string          `json:"committed"`
	Files      map[string]File `json:"files"`
	Skeleton   []string        `json:"skeleton"`
}

// File is one vendored file: its git mode, its sha256, and for a spec file the number of
// cases in it, counted as Oils counts them, one for each line that begins with ####.
type File struct {
	Mode   string `json:"mode"`
	SHA256 string `json:"sha256"`
	Cases  int    `json:"cases"`
}

// ReadUpstream reads upstream.json from the vendored copy at root.
func ReadUpstream(root string) (Upstream, error) {
	data, err := os.ReadFile(filepath.Join(root, "upstream.json"))
	if err != nil {
		return Upstream{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var record Upstream
	if err := decoder.Decode(&record); err != nil {
		return Upstream{}, fmt.Errorf("upstream.json: %w", err)
	}
	return record, nil
}
