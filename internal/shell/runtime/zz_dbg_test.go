package runtime

import "testing"

func TestDbgLogicalLines(t *testing.T) {
	for _, source := range []string{
		"case a in a) echo one ;& b) echo two ;; esac\n",
		"case a in a) echo one ;;& b) echo two ;; esac\n",
	} {
		lines, err := logicalLines(normalizeLineEndings(source))
		t.Logf("source=%q err=%v", source, err)
		for index, line := range lines {
			t.Logf("  [%d] %q", index, line)
		}
		expanded := expandCaseArmLines(lines)
		for index, line := range expanded {
			t.Logf("  case[%d] %q", index, line)
		}
	}
}
