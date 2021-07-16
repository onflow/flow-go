package blocktimer

import "time"

// NoopBlockTimer implements an always valid behavior for BlockTimestamp interface.
// Can be used by nodes that don't perform validation of block timestamps.
type NoopBlockTimer struct{}

func NewNoopBlockTimer() *NoopBlockTimer {
	return &NoopBlockTimer{}
}

func (n NoopBlockTimer) Build(time.Time) time.Time {
	return time.Now().UTC()
}

func (n NoopBlockTimer) Validate(time.Time, time.Time) error {
	return nil
}
