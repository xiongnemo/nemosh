package runtime

import (
	"reflect"
	"testing"
)

func TestEnvironment_preservesExactCaseEmptyValuesAndUnset(t *testing.T) {
	env := NewEnvironment([]string{"PATH=upper", "Path=title", "EMPTY="})

	if value, ok := env.LookupEnv("PATH"); !ok || value != "upper" {
		t.Fatalf("PATH: value=%q present=%t", value, ok)
	}
	if value, ok := env.LookupEnv("Path"); !ok || value != "title" {
		t.Fatalf("Path: value=%q present=%t", value, ok)
	}
	if value, ok := env.LookupEnv("EMPTY"); !ok || value != "" {
		t.Fatalf("EMPTY: value=%q present=%t", value, ok)
	}
	env.Unset("PATH")
	if _, ok := env.LookupEnv("PATH"); ok {
		t.Fatal("PATH remained exported")
	}
	if value, ok := env.LookupEnv("Path"); !ok || value != "title" {
		t.Fatalf("Path changed: value=%q present=%t", value, ok)
	}
}

func TestEnvironment_clonePreservesMutationOrder(t *testing.T) {
	env := NewEnvironment([]string{"Path=first", "OTHER=value"})
	env.Set("PATH", "second")
	clone := env.clone()
	clone.Set("Path", "third")

	wantParent := []string{"OTHER=value", "PATH=second"}
	if got := env.childEnviron(windowsEnvironment); !reflect.DeepEqual(got, wantParent) {
		t.Fatalf("parent: got %v want %v", got, wantParent)
	}
	wantClone := []string{"OTHER=value", "PATH=second"}
	if got := clone.childEnviron(windowsEnvironment); !reflect.DeepEqual(got, wantClone) {
		t.Fatalf("clone: got %v want %v", got, wantClone)
	}
}

func TestEnvironment_childEnvironSerializesDeterministicallyByPlatform(t *testing.T) {
	env := NewEnvironment([]string{"z=last-sort", "PATH=upper", "Empty=", "Path=title"})

	wantUnix := []string{"Empty=", "PATH=upper", "Path=title", "z=last-sort"}
	if got := env.childEnviron(unixEnvironment); !reflect.DeepEqual(got, wantUnix) {
		t.Fatalf("unix: got %v want %v", got, wantUnix)
	}
	wantWindows := []string{"Empty=", "PATH=upper", "z=last-sort"}
	if got := env.childEnviron(windowsEnvironment); !reflect.DeepEqual(got, wantWindows) {
		t.Fatalf("windows: got %v want %v", got, wantWindows)
	}
}

func TestEnvironment_windowsSerializationKeepsCanonicalKnownName(t *testing.T) {
	env := NewEnvironment([]string{"Path=first", "PATH=second"})
	env.Set("Path", "")

	want := []string{"PATH=second"}
	if got := env.childEnviron(windowsEnvironment); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestNewHostEnvironment_windowsCanonicalizesKnownNames_andCanonicalInputWins(t *testing.T) {
	env := newHostEnvironment([]string{
		"Path=title-first",
		"PATH=canonical",
		"pAtH=title-last",
		"ComSpec=command-shell",
	}, windowsEnvironment)

	if value, ok := env.LookupEnv("PATH"); !ok || value != "canonical" {
		t.Fatalf("PATH: value=%q present=%t", value, ok)
	}
	if _, ok := env.LookupEnv("Path"); ok {
		t.Fatal("Path remained in host-imported shell environment")
	}
	if value, ok := env.LookupEnv("COMSPEC"); !ok || value != "command-shell" {
		t.Fatalf("COMSPEC: value=%q present=%t", value, ok)
	}
}

func TestEnvironment_windowsSerializationPrefersCanonicalKnownName_overLaterCaseVariant(t *testing.T) {
	env := NewEnvironment([]string{"PATH=canonical", "Path=title"})
	env.Set("pAtH", "latest")

	want := []string{"PATH=canonical"}
	if got := env.childEnviron(windowsEnvironment); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestEnvironment_windowsSerializationUsesLatestMutation_forUnknownName(t *testing.T) {
	env := NewEnvironment([]string{"Custom=first", "CUSTOM=second"})
	env.Set("custom", "latest")

	want := []string{"custom=latest"}
	if got := env.childEnviron(windowsEnvironment); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRuntime_canonicalParentWinsWindowsSerializationWithoutChangingParent(t *testing.T) {
	runtime := Runtime{vars: map[string]string{}, env: NewEnvironment([]string{"PATH=upper", "Path=title"})}

	child := runtime.withLocalAssignments([]assignment{{name: "pAtH", value: "child"}})

	if child == nil {
		t.Fatal("temporary assignment failed")
	}
	want := []string{"PATH=upper"}
	if got := child.env.childEnviron(windowsEnvironment); !reflect.DeepEqual(got, want) {
		t.Fatalf("child: got %v want %v", got, want)
	}
	if value, ok := runtime.env.LookupEnv("PATH"); !ok || value != "upper" {
		t.Fatalf("parent PATH: value=%q present=%t", value, ok)
	}
	if value, ok := runtime.env.LookupEnv("Path"); !ok || value != "title" {
		t.Fatalf("parent Path: value=%q present=%t", value, ok)
	}
}
