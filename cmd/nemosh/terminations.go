package main

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyTerminations is the signals that would end a script, as numbers, for
// runtime.ReceiveSignals, and the function that stops the notification. Only while a
// script runs: a prompt ignores TERM, as bash's does, and on Windows a console-close
// handler that nothing answered would hold the closing window open.
func notifyTerminations() (<-chan int, func()) {
	raw := make(chan os.Signal, 4)
	signal.Notify(raw, terminationSignals...)
	numbers := make(chan int, 4)
	done := make(chan struct{})
	go func() {
		for {
			select {
			case received := <-raw:
				if number, ok := received.(syscall.Signal); ok {
					select {
					case numbers <- int(number):
					default:
					}
				}
			case <-done:
				return
			}
		}
	}()
	return numbers, func() {
		signal.Stop(raw)
		close(done)
	}
}

// signalExit is a script a signal ended, which main ends the process by.
type signalExit int

func (s signalExit) Error() string { return "ended by signal " + syscall.Signal(s).String() }
