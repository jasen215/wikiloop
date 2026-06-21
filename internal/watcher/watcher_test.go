package watcher

import (
	"sync/atomic"
	"testing"
	"time"
)

// TestDebounce verifies that multiple rapid Trigger calls result in exactly
// one callback invocation — the debouncer should coalesce all calls within
// the window into a single fire.
func TestDebounce(t *testing.T) {
	var count atomic.Int32
	d := NewDebouncer(80*time.Millisecond, func() {
		count.Add(1)
	})

	// Fire 3 times rapidly (well within debounce window)
	d.Trigger()
	d.Trigger()
	d.Trigger()

	// Wait longer than debounce duration for the callback to fire
	time.Sleep(200 * time.Millisecond)

	if got := count.Load(); got != 1 {
		t.Errorf("expected callback to fire exactly 1 time, got %d", got)
	}
}

func TestDebounce_SerializesCallbacks(t *testing.T) {
	var count atomic.Int32
	var concurrent atomic.Int32
	var maxConcurrent atomic.Int32
	started := make(chan struct{}, 1)
	release := make(chan struct{})

	d := NewDebouncer(20*time.Millisecond, func() {
		current := concurrent.Add(1)
		for {
			max := maxConcurrent.Load()
			if current <= max || maxConcurrent.CompareAndSwap(max, current) {
				break
			}
		}
		n := count.Add(1)
		if n == 1 {
			started <- struct{}{}
			<-release
		}
		concurrent.Add(-1)
	})

	d.Trigger()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first callback did not start")
	}

	// These events arrive while the first callback is still running. They
	// should become one follow-up callback, never overlapping the first.
	d.Trigger()
	d.Trigger()
	d.Trigger()
	close(release)

	deadline := time.Now().Add(time.Second)
	for count.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := count.Load(); got != 2 {
		t.Fatalf("callback count = %d, want 2", got)
	}
	if got := maxConcurrent.Load(); got != 1 {
		t.Fatalf("max concurrent callbacks = %d, want 1", got)
	}
}

func TestGeneratedWikiPathsAreIgnored(t *testing.T) {
	wikiDir := "/tmp/kb/wiki"
	tests := []struct {
		path string
		want bool
	}{
		{path: "/tmp/kb/wiki/index.md", want: true},
		{path: "/tmp/kb/wiki/log.md", want: true},
		{path: "/tmp/kb/wiki/source-notes/index.md", want: true},
		{path: "/tmp/kb/wiki/source-notes/topic.md", want: false},
		{path: "/tmp/kb/raw/index.md", want: false},
	}
	for _, tt := range tests {
		if got := isGeneratedWikiPath(tt.path, wikiDir); got != tt.want {
			t.Errorf("isGeneratedWikiPath(%q) = %v, want %v", tt.path, got, tt.want)
		}
	}
}

// TestDebounce_ResetOnNewTrigger verifies that a second Trigger before the
// debounce window expires resets the timer, so the callback fires once
// measured from the LAST trigger, not the first.
func TestDebounce_ResetOnNewTrigger(t *testing.T) {
	var count atomic.Int32
	d := NewDebouncer(80*time.Millisecond, func() {
		count.Add(1)
	})

	// First trigger
	d.Trigger()
	// Wait 50ms — timer not yet expired (80ms window)
	time.Sleep(50 * time.Millisecond)
	// Second trigger resets the window
	d.Trigger()
	// Wait 120ms — past the 80ms window from the second trigger
	time.Sleep(120 * time.Millisecond)

	if got := count.Load(); got != 1 {
		t.Errorf("expected callback to fire exactly 1 time after reset, got %d", got)
	}
}
