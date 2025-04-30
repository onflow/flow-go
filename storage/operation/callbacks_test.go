package operation

import "testing"

func TestCallback(t *testing.T) {
	cb := NewCallbacks()
	var called bool
	cb.AddCallback(func(err error) {
		called = true
	})
	cb.NotifyCallbacks(nil)
	if !called {
		t.Error("Callback was not called")
	}
}

func TestCallbackConcurrency(t *testing.T) {
	cb := NewCallbacks()
	var called bool
	cb.AddCallback(func(err error) {
		called = true
	})
	done := make(chan struct{})
	go func() {
		cb.NotifyCallbacks(nil)
		close(done)
	}()
	<-done
	if !called {
		t.Error("Callback was not called")
	}
}
