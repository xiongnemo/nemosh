// Copyright 2015 Garrett D'Amore
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use file except in compliance with the License.
// You may obtain a copy of the license at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Modified for nemosh: the table is generated into tables.go rather than built in an
// init, where every program linking this package paid for it at start; the map it was
// built from is in charmap_source_test.go. See README.md.

package encoding

import (
	"golang.org/x/text/encoding"
)

// EBCDIC represents the 8-bit EBCDIC scheme, found in some mainframe
// environments.  If you don't know what this is, consider yourself lucky.
var EBCDIC encoding.Encoding = &ebcdicCharmap
