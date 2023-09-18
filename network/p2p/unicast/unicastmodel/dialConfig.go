package unicastmodel

import "time"

const (
	// MaxConnectAttempt is the maximum number of attempts to be made to connect to a remote node to establish a unicast (1:1) connection.
	MaxConnectAttempt = 3

	// MaxStreamCreationAttempt is the maximum number of attempts to be made to create a stream to a remote node over a direct unicast (1:1) connection.
	MaxStreamCreationAttempt = 3

	// PeerReliabilityThreshold is the threshold for the dial history to a remote peer that is considered reliable. When
	// the last time we dialed a peer is less than this threshold, we will assume the remote peer is not reliable. Otherwise,
	// we will assume the remote peer is reliable.
	//
	// For example, with PeerReliabilityThreshold set to 5 minutes, if the last time we dialed a peer was 5 minutes ago, we will
	// assume the remote peer is reliable and will attempt to dial again with more flexible dial options. However, if the last time
	// we dialed a peer was 3 minutes ago, we will assume the remote peer is not reliable and will attempt to dial again with
	// more strict dial options.
	//
	// Note in Flow, we only dial a peer when we need to send a message to it; and we assume two nodes that are supposed to
	// exchange unicast messages will do it frequently. Therefore, the dial history is a good indicator
	// of the reliability of the peer.
	PeerReliabilityThreshold = 5 * time.Minute
)

// DialConfig is a struct that represents the dial config for a peer.
type DialConfig struct {
	DialBackoffBudget  uint64 // number of times we have to try to dial the peer before we give up.
	StreamBackBudget   uint64 // number of times we have to try to open a stream to the peer before we give up.
	LastSuccessfulDial uint64 // timestamp of the last successful dial to the peer.
}

// DialConfigAdjustFunc is a function that is used to adjust the fields of a DialConfigEntity.
// The function is called with the current config and should return the adjusted record.
// Returned error indicates that the adjustment is not applied, and the config should not be updated.
// In BFT setup, the returned error should be treated as a fatal error.
type DialConfigAdjustFunc func(DialConfig) (DialConfig, error)

func DefaultDialConfigFactory() DialConfig {
	return DialConfig{
		DialBackoffBudget: MaxConnectAttempt,
		StreamBackBudget:  MaxStreamCreationAttempt,

		// TODO: test it!
		LastSuccessfulDial: 0,
	}
}
