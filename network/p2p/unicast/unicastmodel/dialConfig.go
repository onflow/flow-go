package unicastmodel

import "time"

const (
	// MaxConnectAttempt is the maximum number of attempts to be made to connect to a remote node to establish a unicast (1:1) connection.
	MaxConnectAttempt = 3

	// MaxStreamCreationAttempt is the maximum number of attempts to be made to create a stream to a remote node over a direct unicast (1:1) connection.
	MaxStreamCreationAttempt = 3

	// StreamZeroBackoffResetThreshold is the threshold that determines when to reset the stream creation backoff budget to the default value.
	//
	// For example the threshold of 100 means that if the stream creation backoff budget is decreased to 0, then it will be reset to default value
	// when the number of consecutive successful streams reaches 100.
	//
	// This is to prevent the backoff budget from being reset too frequently, as the backoff budget is used to gauge the reliability of the stream creation.
	// When the stream creation backoff budget is reset to the default value, it means that the stream creation is reliable enough to be trusted again.
	// This parameter mandates when the stream creation is reliable enough to be trusted again; i.e., when the number of consecutive successful streams reaches this threshold.
	// Note that the counter is reset to 0 when the stream creation fails, so the value of for example 100 means that the stream creation is reliable enough that the recent
	// 100 stream creations are all successful.
	StreamZeroBackoffResetThreshold = 100

	// DialZeroBackoffResetThreshold is the threshold that determines when to reset the dial backoff budget to the default value.
	//
	// For example the threshold of 1 hour means that if the dial backoff budget is decreased to 0, then it will be reset to default value
	// when it has been 1 hour since the last successful dial.
	//
	// This is to prevent the backoff budget from being reset too frequently, as the backoff budget is used to gauge the reliability of the dialing a remote peer.
	// When the dial backoff budget is reset to the default value, it means that the dialing is reliable enough to be trusted again.
	// This parameter mandates when the dialing is reliable enough to be trusted again; i.e., when it has been 1 hour since the last successful dial.
	// Note that the last dial attempt timestamp is reset to zero when the dial fails, so the value of for example 1 hour means that the dialing to the remote peer is reliable enough that the last
	// successful dial attempt was 1 hour ago.
	DialZeroBackoffResetThreshold = 1 * time.Hour
)

// DialConfig is a struct that represents the dial config for a peer.
type DialConfig struct {
	DialBackoffBudget           uint64    // number of times we have to try to dial the peer before we give up.
	StreamBackBudget            uint64    // number of times we have to try to open a stream to the peer before we give up.
	LastSuccessfulDial          time.Time // timestamp of the last successful dial to the peer.
	ConsecutiveSuccessfulStream uint64    // consecutive number of successful streams to the peer since the last time stream creation failed.
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
	}
}
