package topology

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

type TopicAwareTopology struct {
	RandPermTopology
	roleByTopic map[string]flow.RoleList // used to map between topics and roles subscribed to topics
}

// Subset receives an identity list and a topic. It then extracts list of nodes subscribed to the topic from the
// identity list, and samples and returns`(k+1)/2` of them uniformly where `k` is the number of nodes subscribed to the
// topic.
func (t *TopicAwareTopology) Subset(idList flow.IdentityList, _ uint, topic string) (flow.IdentityList, error) {
	// extracts flow roles subscribed to topic.
	roles, ok := t.roleByTopic[topic]
	if !ok {
		return nil, fmt.Errorf("unknown topic with no subscribed roles: %s", topic)
	}

	// extracts node ids with specified roles.
	ids := idList.Filter(filter.HasRole(roles...))

	// determines fanout concerning connectedness.
	fanout := uint((len(ids) + 1) / 2)

	// samples uniformly among the ids of specified topic.
	randomSample, err := t.RandPermTopology.Subset(ids, fanout, DummyTopic)
	if err != nil {
		return nil, fmt.Errorf("could not sample from subscribed nodes to the topic: %w", err)
	}

	return randomSample, nil
}
