package runtime

import (
	"bytes"
	"context"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

// The shapes a script spends its time in, a thousand turns each, for profiling the
// interpreter: `go test -run '^$' -bench BenchmarkLoop -cpuprofile cpu.out`. busybox-w32's ash
// ran each of them in about half the time when these were written.
func BenchmarkLoop(b *testing.B) {
	for _, loop := range []struct{ name, script string }{
		{"function", "f(){ :; }; i=0; while [ $i -lt 1000 ]; do f; i=$((i+1)); done\n"},
		{"inline", "i=0; while [ $i -lt 1000 ]; do :; i=$((i+1)); done\n"},
		{"arithmetic", "i=0; while [ $i -lt 1000 ]; do : $((i+=1)); done\n"},
		{"assignment", "i=0; while [ $i -lt 1000 ]; do x=$i; y=\"$x-$i\"; i=$((i+1)); done\n"},
	} {
		b.Run(loop.name, func(b *testing.B) {
			var output bytes.Buffer
			b.ReportAllocs()
			for b.Loop() {
				rt := New(applets.DefaultRegistry, Streams{Stdout: &output, Stderr: &output})
				if status := rt.RunScript(context.Background(), loop.script); status != 0 {
					b.Fatalf("exited %d: %s", status, output.String())
				}
			}
		})
	}
}
