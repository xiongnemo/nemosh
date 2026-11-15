package runtime

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/xiongnemo/nemosh/internal/applets"
)

func frameTestRuntime(stdout *bytes.Buffer) Runtime {
	return New(applets.DefaultRegistry, Streams{Stdin: strings.NewReader(""), Stdout: stdout, Stderr: &bytes.Buffer{}})
}

func TestAppletFrames_handTheSameCallTheSameFrame(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	rt := frameTestRuntime(&bytes.Buffer{})
	frames := &appletFrames{}

	// When
	first := frames.frameFor(ctx, &rt)
	second := frames.frameFor(ctx, &rt)

	// Then
	if first != second {
		t.Fatal("a repeated call was made a second frame")
	}
}

func TestAppletFrames_makeANewFrameForAnyOtherCall(t *testing.T) {
	// Given
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	other, cancelOther := context.WithCancel(context.Background())
	defer cancelOther()
	rt := frameTestRuntime(&bytes.Buffer{})
	redirected := rt
	redirected.streams.Stdout = &bytes.Buffer{}
	frames := &appletFrames{}
	first := frames.frameFor(ctx, &rt)

	for _, test := range []struct {
		name string
		ctx  context.Context
		rt   Runtime
	}{
		{name: "another context", ctx: other, rt: rt},
		{name: "another stdout", ctx: ctx, rt: redirected},
	} {
		t.Run(test.name, func(t *testing.T) {
			// When
			frame := frames.frameFor(test.ctx, &test.rt)

			// Then: a frame that answers as the call would have it
			if frame == first {
				t.Fatal("the call was handed a frame made for another")
			}
			if _, stdout := frame.streams(); stdout.(*interruptibleWriter).writer != test.rt.streams.Stdout {
				t.Fatal("the frame writes somewhere other than the call's stdout")
			}
		})
	}
}

func TestAppletFrames_areNotKeptUnderTheFreshSwitch(t *testing.T) {
	// Given
	t.Setenv("NEMOSH_APPLET_FRAMES", "fresh")
	rt := frameTestRuntime(&bytes.Buffer{})
	frames := newAppletFrames()

	// When
	first := frames.frameFor(context.Background(), &rt)
	second := frames.frameFor(context.Background(), &rt)

	// Then
	if first == second {
		t.Fatal("NEMOSH_APPLET_FRAMES=fresh handed one frame to two calls")
	}
}

// Goroutines -- a pipeline's stages, a background job -- call in contexts of their own at
// once, and each must be handed a frame for its own call, never one made for another's.
func TestAppletFrames_handEachGoroutineAFrameForItsOwnCall(t *testing.T) {
	// Given
	rt := frameTestRuntime(&bytes.Buffer{})
	frames := &appletFrames{}
	var wait sync.WaitGroup

	// When
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			for range 200 {
				if frame := frames.frameFor(ctx, &rt); frame.parent != ctx {
					t.Error("a goroutine was handed a frame made for another's context")
					return
				}
			}
		}()
	}

	// Then
	wait.Wait()
}

func TestAppletFrame_answersForTheShellAndPassesTheRestOn(t *testing.T) {
	// Given
	type probeKey struct{}
	parent, cancel := context.WithCancel(context.WithValue(context.Background(), probeKey{}, "carried"))
	rt := frameTestRuntime(&bytes.Buffer{})

	// When
	frame := (&appletFrames{}).frameFor(parent, &rt)
	derived, release := context.WithCancel(frame)
	defer release()

	// Then: the shell, as the view the applets ask for
	if _, ok := applets.ProcessViewFromContext(frame).(*Runtime); !ok {
		t.Fatalf("the frame's ProcessView is %T", applets.ProcessViewFromContext(frame))
	}
	if got := frame.Value(probeKey{}); got != "carried" {
		t.Fatalf("a key of the caller's answered %v", got)
	}
	// and a context an applet derives from it ends when the call's does
	cancel()
	<-derived.Done()
	if frame.Err() == nil {
		t.Fatal("the frame did not report its call's cancellation")
	}
}
