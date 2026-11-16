package main

import (
	"context"
	"os"
	"os/signal"
	"sync"

	"github.com/xiongnemo/nemosh/internal/shell/runtime"
)

func notifyInterrupts() (<-chan os.Signal, func()) {
	interrupts := make(chan os.Signal, 1)
	signal.Notify(interrupts, os.Interrupt)
	return interrupts, func() { signal.Stop(interrupts) }
}

type interruptController struct {
	mu          sync.Mutex
	active      *interruptSlot
	idlePending bool
	idleWake    chan struct{}
	// console says an interrupt is the console's Ctrl-C, which the program in the foreground
	// had as well, so one run from the prompt may be left to handle it; see the runtime's
	// external_cancel.go. Only main sets it, and only on Windows: a test's interrupt reached
	// nothing else.
	console bool
}

type interruptSlot struct {
	interrupt func()
	finish    func()
}

// begin is the context a line typed at the prompt runs in; context is a script's.
func (c *interruptController) begin(parent context.Context) (context.Context, func(), bool) {
	return c.start(parent, true)
}

func (c *interruptController) start(parent context.Context, prompt bool) (context.Context, func(), bool) {
	ctx, interrupt, release := runtime.InterruptContextWithRelease(parent)
	if c.console {
		ctx, interrupt, release = runtime.ConsoleInterruptContextWithRelease(parent, prompt)
	}
	slot := &interruptSlot{interrupt: interrupt, finish: release}
	c.mu.Lock()
	idlePending := c.consumeIdleInterruptLocked()
	if !idlePending {
		c.active = slot
	}
	c.mu.Unlock()
	if idlePending {
		interrupt()
	}
	return ctx, func() {
		c.mu.Lock()
		slot.finish()
		if c.active == slot {
			c.active = nil
		}
		c.mu.Unlock()
	}, idlePending
}

func (c *interruptController) context(parent context.Context) (context.Context, func()) {
	ctx, finish, _ := c.start(parent, false)
	return ctx, finish
}

func (c *interruptController) interrupt() {
	c.mu.Lock()
	if c.active == nil {
		c.idlePending = true
		wake := c.idleInterruptsLocked()
		select {
		case wake <- struct{}{}:
		default:
		}
		c.mu.Unlock()
		return
	}
	c.active.interrupt()
	c.mu.Unlock()
}

func (c *interruptController) idleInterrupts() <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.idleInterruptsLocked()
}

func (c *interruptController) idleInterruptsLocked() chan struct{} {
	if c.idleWake == nil {
		c.idleWake = make(chan struct{}, 1)
	}
	return c.idleWake
}

func (c *interruptController) consumeIdleInterrupt() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.consumeIdleInterruptLocked()
}

func (c *interruptController) consumeIdleInterruptLocked() bool {
	if !c.idlePending {
		return false
	}
	c.idlePending = false
	if c.idleWake != nil {
		select {
		case <-c.idleWake:
		default:
		}
	}
	return true
}
