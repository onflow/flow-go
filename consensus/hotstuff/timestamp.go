package hotstuff

import (
	"fmt"
	"time"
)

type BlockTimestamp struct {
	minInterval time.Duration
	maxInterval time.Duration
}

func NewBlockTimestamp(minInterval, maxInterval time.Duration) *BlockTimestamp {
	return &BlockTimestamp{
		minInterval: minInterval,
		maxInterval: maxInterval,
	}
}

func (b BlockTimestamp) Build(parentTimestamp time.Time) time.Time {
	// calculate the timestamp and cutoffs
	timestamp := time.Now().UTC()
	from := parentTimestamp.Add(b.minInterval)
	to := parentTimestamp.Add(b.maxInterval)

	// adjust timestamp if outside of cutoffs
	if timestamp.Before(from) {
		timestamp = from
	}
	if timestamp.After(to) {
		timestamp = to
	}

	return timestamp
}

func (b BlockTimestamp) Validate(parentTimestamp, currentTimestamp time.Time) error {
	from := parentTimestamp.Add(b.minInterval)
	to := parentTimestamp.Add(b.maxInterval)
	if currentTimestamp.Before(from) {
		return fmt.Errorf("invalid timestamp")
	}
	if currentTimestamp.After(to) {
		return fmt.Errorf("invalid timestamp")
	}
	return nil
}
