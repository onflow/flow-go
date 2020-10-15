package topology

import (
	"fmt"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

type TopicAwareTopology struct {
	RandPermTopology
	seed int64
}

// NewTopicAwareTopology returns an instance of the TopicAwareTopology
func NewTopicAwareTopology(myId flow.Identifier) (*TopicAwareTopology, error) {
	seed, err := seedFromID(myId)
	if err != nil {
		return nil, fmt.Errorf("failed to seed topology: %w", err)
	}
	t := &TopicAwareTopology{
		seed: seed,
	}

	return t, nil
}

// Subset samples and returns a connected graph fanout of the subscribers to the topic from the idList.
// A connected graph fanout means that the subset of ids returned by this method on different nodes collectively
// construct a connected graph component among all the subscribers to the topic.
func (t *TopicAwareTopology) Subset(idList flow.IdentityList, _ uint, topic string) (flow.IdentityList, error) {
	// extracts flow roles subscribed to topic.
	roles, ok := engine.GetRolesByTopic(topic)
	if !ok {
		return nil, fmt.Errorf("unknown topic with no subscribed roles: %s", topic)
	}

	// extract ids of subscribers to the topic
	subscribers := idList.Filter(filter.HasRole(roles...))

	// samples subscribers of a connected graph
	subscriberSample, _ := connectedGraphSample(subscribers, t.seed)

	return subscriberSample, nil
}
