package runtime

import "testing"

func TestScratchLogicalLines(t *testing.T) {
	for _, source := range []string{
		"if { true; }; then echo g; fi\n",
		"if case a in a) true;; esac; then echo c; fi\n",
		"if ( true ); then echo s; fi\n",
		"if for i in 1; do true; done; then echo f; fi\n",
		"echo a | case a in a) echo x;; esac\n",
	} {
		lines, err := logicalLines(source)
		if err != nil {
			t.Logf("%q -> error %v", source, err)
			continue
		}
		t.Logf("%q", source)
		for index, line := range lines {
			t.Logf("    [%d] %q", index, line)
		}
	}
}
