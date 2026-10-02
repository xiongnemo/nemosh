package runtime_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

// A function may call itself 1000 deep, as busybox-w32 and bash both let it. It stopped at 128
// with "function call depth exceeds 128". Measured against both.
func TestRuntime_aFunctionRecursesAThousandDeep(t *testing.T) {
	var stdout, stderr bytes.Buffer
	rt := runtime.New(applets.DefaultRegistry, runtime.Streams{Stdout: &stdout, Stderr: &stderr})
	status := rt.RunScript(context.Background(), "f() { [ $1 -gt 0 ] && f $(($1-1)); }; f 1000; echo done\n")
	rt.CloseBatch(status)
	if stdout.String() != "done\n" || stderr.String() != "" {
		t.Errorf("recursing 1000 deep = %q, %q; want done", stdout.String(), stderr.String())
	}
}
