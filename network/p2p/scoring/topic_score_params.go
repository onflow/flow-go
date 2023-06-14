package scoring

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

const (
	defaultSkipAtomicValidation = true
	// defaultInvalidMessageDeliveriesWeight the default weight for invalid message delivery penalization. With a default value of 10, 3 invalid message deliveries
	// will result in the nodes peer score decreasing to -1000 (-10^3) this will result in the connection to the node being pruned.
	defaultInvalidMessageDeliveriesWeight = -100
	// defaultInvalidMessageDeliveriesDecay decay factor used to decay the number of invalid message deliveries.
	defaultInvalidMessageDeliveriesDecay = .99
	defaultTimeInMeshQuantum             = time.Second
)

func init() {

}

// defaultTopicScoreParams returns the default score params for topics. Currently, these params only set invalid message delivery score params.
func defaultTopicScoreParams() *pubsub.TopicScoreParams {
	return &pubsub.TopicScoreParams{
		SkipAtomicValidation:           defaultSkipAtomicValidation,
		InvalidMessageDeliveriesWeight: defaultInvalidMessageDeliveriesWeight,
		InvalidMessageDeliveriesDecay:  defaultInvalidMessageDeliveriesDecay,
		TimeInMeshQuantum:              defaultTimeInMeshQuantum,
	}
}

// DefaultTopicScoreParams  returns the default topic score parameters.
func DefaultTopicScoreParams(sporkId flow.Identifier) map[channels.Topic]*pubsub.TopicScoreParams {
	topicScoreParams := make(map[channels.Topic]*pubsub.TopicScoreParams)
	topicScoreParams[channels.TopicFromChannel(channels.PushBlocks, sporkId)] = defaultTopicScoreParams()
	return topicScoreParams
}
