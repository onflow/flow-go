package timestamp

import "time"

// NoopBlockTimestamp implements an always valid behavior for BlockTimestamp interface.
// Can be used by nodes that don't perform validation of block timestamps.
type NoopBlockTimestamp struct{}

func NewNoopBlocktimer() *NoopBlocktimer {
	return &NoopBlockTimestamp{}
}

func (n NoopBlockTimestamp) Build(time.Time) time.Time {
	return time.Now().UTC()
}

func (n NoopBlockTimestamp) Validate(time.Time, time.Time) error {
	return nil
}
