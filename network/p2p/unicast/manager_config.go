package unicast

import (
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network/p2p"
)

type ManagerConfig struct {
	Logger        zerolog.Logger               `validate:"required"`
	StreamFactory p2p.StreamFactory            `validate:"required"`
	SporkId       flow.Identifier              `validate:"required"`
	Metrics       module.UnicastManagerMetrics `validate:"required"`

	// CreateStreamBackoffDelay is the backoff delay between retrying stream creations to the same peer.
	CreateStreamBackoffDelay time.Duration `validate:"gt=0"`

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
	StreamZeroRetryResetThreshold uint64 `validate:"gt=0"`

	// MaxStreamCreationRetryAttemptTimes is the maximum number of attempts to be made to create a stream to a remote node over a direct unicast (1:1) connection before we give up.
	MaxStreamCreationRetryAttemptTimes uint64 `validate:"gt=0"`

	// UnicastConfigCacheFactory is a factory function to create a new dial config cache.
	UnicastConfigCacheFactory DialConfigCacheFactory `validate:"required"`
}
