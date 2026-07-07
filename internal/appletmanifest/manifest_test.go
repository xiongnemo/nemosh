package appletmanifest

import (
	"reflect"
	"testing"
)

func TestParseBusyBoxApplets_returnsNames_whenMacroVariantsAreDeclared(t *testing.T) {
	// Given: BusyBox-w32 source text with representative //applet: macro variants.
	source := `
//applet:IF_TRUE(APPLET(true, BB_DIR_BIN, BB_SUID_DROP))
//applet:IF_OPEN(APPLET_ODDNAME([, test, BB_DIR_USR_BIN, BB_SUID_DROP, test))
//applet:IF_ECHO(APPLET_NOEXEC(echo, echo, BB_DIR_BIN, BB_SUID_DROP, echo))
//applet:IF_TEST(APPLET_NOFORK(test, test, BB_DIR_USR_BIN, BB_SUID_DROP, test))
//applet:IF_ASH(APPLET_SCRIPTED(winpath, winpath, BB_DIR_USR_BIN, BB_SUID_DROP, winpath))
`

	// When: the BusyBox declarations are parsed.
	got := ParseBusyBoxApplets(source)

	// Then: the declared command names are returned in declaration order.
	want := []string{"true", "[", "echo", "test", "winpath"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseBusyBoxApplets() = %v, want %v", got, want)
	}
}

func TestParseNemoshRegistry_returnsNames_whenDefaultRegistryContainsConstructors(t *testing.T) {
	// Given: Nemosh registry source text with default registry constructor calls.
	source := `
var DefaultRegistry = NewRegistry(
	newTrueApplet(),
	newTestApplet("["),
	newWinpathApplet(),
	newPosixpathApplet(),
)
`

	// When: the registry source text is parsed.
	got := ParseNemoshRegistry(source)

	// Then: constructor calls are mapped to registered applet names in source order.
	want := []string{"true", "[", "winpath", "posixpath"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseNemoshRegistry() = %v, want %v", got, want)
	}
}

func TestCompareApplets_classifiesNames_whenInputsOverlap(t *testing.T) {
	// Given: BusyBox declares shared commands plus commands missing from Nemosh.
	busyboxNames := []string{"true", "[", "echo", "winpath", "missing-busybox"}
	nemoshNames := []string{"[", "posixpath", "true", "winpath", "nemosh-only"}

	// When: the two name manifests are compared.
	got := CompareApplets(busyboxNames, nemoshNames)

	// Then: names are classified into deterministic implemented, missing, and Nemosh-only sets.
	want := AppletComparison{
		Implemented: []string{"[", "true", "winpath"},
		Missing:     []string{"echo", "missing-busybox"},
		NemoshOnly:  []string{"nemosh-only", "posixpath"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("CompareApplets() = %#v, want %#v", got, want)
	}
}
