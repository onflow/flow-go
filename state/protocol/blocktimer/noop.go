package blocktimer

import "time"

// NoopBlockTimer implements an always valid behavior for BlockTimestamp interface.
// Can be used by nodes that don't perform validation of block timestamps.
type NoopBlockTimer struct{}

func NewNoopBlockTimer() *NoopBlockTimer {
	return &NoopBlockTimer{}
}

func (n NoopBlockTimer) Build(uint64) uint64 {
	return uint64(time.Now().UnixMilli())
}

func (n NoopBlockTimer) Validate(uint64, uint64) error {
	return nil
}
