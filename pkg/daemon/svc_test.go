package daemon

import (
	"context"
	"testing"
	"time"
)

func TestProgramStartIsNonBlockingAndStopCancels(t *testing.T) {
	started := make(chan struct{})
	releasedCtx := make(chan struct{})

	p := newProgram(Options{BinaryName: "t"}.withDefaults())
	// Replace the supervisor body with a controllable blocker.
	p.run = func(ctx context.Context, _ Options) error {
		close(started)
		<-ctx.Done() // unblocks only when Stop cancels
		close(releasedCtx)
		return ctx.Err()
	}

	// Start must NOT block: it launches the supervisor in a goroutine.
	if err := p.Start(nil); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not launch the supervisor goroutine")
	}

	// Stop must cancel the context and return well within the budget.
	stopReturned := make(chan error, 1)
	go func() { stopReturned <- p.Stop(nil) }()
	select {
	case err := <-stopReturned:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Stop did not return within budget")
	}
	select {
	case <-releasedCtx:
	case <-time.After(time.Second):
		t.Fatal("supervisor context was not cancelled by Stop")
	}
}

func TestProgramStopWaitsThenGivesUp(t *testing.T) {
	// A supervisor that ignores cancellation: Stop must still return (after the
	// time.After fallback fires), not hang forever.
	p := newProgram(Options{BinaryName: "t"}.withDefaults())
	p.stopTimeout = 50 * time.Millisecond // shrink the budget for the test
	p.run = func(ctx context.Context, _ Options) error {
		<-make(chan struct{}) // block forever, ignoring ctx
		return nil
	}
	_ = p.Start(nil)

	done := make(chan error, 1)
	go func() { done <- p.Stop(nil) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Stop returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not honour its fallback timeout")
	}
}
