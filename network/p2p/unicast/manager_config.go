package unicast

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
)

type ManagerConfig struct {
	Logger        zerolog.Logger
	StreamFactory p2p.StreamFactory
	SporkId       flow.Identifier
	ConnStatus    p2p.PeerConnections
	Metrics       module.UnicastManagerMetrics

	// CreateStreamBackoffDelay is the backoff delay between retrying stream creations to the same peer.
	CreateStreamBackoffDelay time.Duration

	// DialInProgressBackoffDelay is the backoff delay for parallel attempts on dialing to the same peer.
	// When the unicast manager is invoked to create stream to the same peer concurrently while there is
	// already an ongoing dialing attempt to the same peer, the unicast manager will wait for this backoff delay
	// and retry creating the stream after the backoff delay has elapsed. This is to prevent the unicast manager
	// from creating too many parallel dialing attempts to the same peer.
	DialInProgressBackoffDelay time.Duration

	// DialBackoffDelay is the backoff delay between retrying connection to the same peer.
	DialBackoffDelay time.Duration

	// StreamZeroRetryResetThreshold is the threshold that determines when to reset the stream creation retry budget to the default value.
	//
	// For example the default value of 100 means that if the stream creation retry budget is decreased to 0, then it will be reset to default value
	// when the number of consecutive successful streams reaches 100.
	//
	// This is to prevent the retry budget from being reset too frequently, as the retry budget is used to gauge the reliability of the stream creation.
	// When the stream creation retry budget is reset to the default value, it means that the stream creation is reliable enough to be trusted again.
	// This parameter mandates when the stream creation is reliable enough to be trusted again; i.e., when the number of consecutive successful streams reaches this threshold.
	// Note that the counter is reset to 0 when the stream creation fails, so the value of for example 100 means that the stream creation is reliable enough that the recent
	// 100 stream creations are all successful.
	StreamZeroRetryResetThreshold uint64

	// DialZeroRetryResetThreshold is the threshold that determines when to reset the dial retry budget to the default value.
	// For example the threshold of 1 hour means that if the dial retry budget is decreased to 0, then it will be reset to default value
	// when it has been 1 hour since the last successful dial.
	//
	// This is to prevent the retry budget from being reset too frequently, as the retry budget is used to gauge the reliability of the dialing a remote peer.
	// When the dial retry budget is reset to the default value, it means that the dialing is reliable enough to be trusted again.
	// This parameter mandates when the dialing is reliable enough to be trusted again; i.e., when it has been 1 hour since the last successful dial.
	// Note that the last dial attempt timestamp is reset to zero when the dial fails, so the value of for example 1 hour means that the dialing to the remote peer is reliable enough that the last
	// successful dial attempt was 1 hour ago.
	DialZeroRetryResetThreshold time.Duration

	// MaxDialRetryAttemptTimes is the maximum number of attempts to be made to connect to a remote node to establish a unicast (1:1) connection before we give up.
	MaxDialRetryAttemptTimes uint64

	// MaxStreamCreationRetryAttemptTimes is the maximum number of attempts to be made to create a stream to a remote node over a direct unicast (1:1) connection before we give up.
	MaxStreamCreationRetryAttemptTimes uint64

	// DialConfigCacheFactory is a factory function to create a new dial config cache.
	DialConfigCacheFactory DialConfigCacheFactory
}
