package internal

import (
	"time"
)

// duplicateMessagesCounter cache record that keeps track of the amount of duplicate messages received from a peer.
type duplicateMessagesCounter struct {
	// Value the number of duplicate messages.
	Value       float64
	lastUpdated time.Time
}

func newDuplicateMessagesCounter() *duplicateMessagesCounter {
	return &duplicateMessagesCounter{
		Value:       0.0,
		lastUpdated: time.Now(),
	}
}
