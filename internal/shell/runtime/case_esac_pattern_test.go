package runtime

import (
	"strings"
	"testing"
)

// An unquoted esac where a pattern goes closes the case, in busybox and bash both, so the `)`
// after it is a syntax error. It was taken for a pattern, and `case esac in esac) echo e;;
// esac` said e. Quoted, or behind the open parenthesis POSIX allows, it is a pattern.
func TestCase_anEsacWhereAPatternGoesClosesTheCase(t *testing.T) {
	for _, script := range []string{
		"case x in esac) echo e;; esac\n",
		"case esac in esac) echo e;; esac\n",
		"case x in a) ;; esac) echo e;; esac\n",
	} {
		stdout, stderr, status := runKill(t, script)
		if stdout != "" || status != 2 || !strings.Contains(stderr, "syntax error: unexpected )") {
			t.Errorf("%q: stdout %q, stderr %q, status %d; want a syntax error", script, stdout, stderr, status)
		}
	}
	for script, want := range map[string]string{
		"case esac in (esac) echo paren;; esac\n": "paren\n",
		"case esac in \"esac\") echo q;; esac\n":  "q\n",
		"case esac in x|esac) echo m;; esac\n":    "m\n",
	} {
		if stdout, stderr, _ := runKill(t, script); stdout != want || stderr != "" {
			t.Errorf("%q: stdout %q, stderr %q; want %q", script, stdout, stderr, want)
		}
	}
}
