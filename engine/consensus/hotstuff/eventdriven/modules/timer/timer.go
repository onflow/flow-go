package timer

import "time"

type Timer struct {
	Timer   *time.Timer
	Timeout time.Duration
}

func (t *Timer) resetTimeout(timeout time.Duration) {
	t.Timeout = timeout
}

func (t *Timer) ResetTimer() {
	if t.Timer != nil {
		t.Timer.Stop()
	}
	t.Timer = time.NewTimer(t.Timeout)
}
