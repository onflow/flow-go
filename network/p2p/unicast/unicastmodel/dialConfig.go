package unicastmodel

import "time"

// DialConfig is a struct that represents the dial config for a peer.
type DialConfig struct {
	DialRetryAttemptBudget           uint64    // number of times we have to try to dial the peer before we give up.
	StreamCreationRetryAttemptBudget uint64    // number of times we have to try to open a stream to the peer before we give up.
	LastSuccessfulDial               time.Time // timestamp of the last successful dial to the peer.
	ConsecutiveSuccessfulStream      uint64    // consecutive number of successful streams to the peer since the last time stream creation failed.
}

// DialConfigAdjustFunc is a function that is used to adjust the fields of a DialConfigEntity.
// The function is called with the current config and should return the adjusted record.
// Returned error indicates that the adjustment is not applied, and the config should not be updated.
// In BFT setup, the returned error should be treated as a fatal error.
type DialConfigAdjustFunc func(DialConfig) (DialConfig, error)
