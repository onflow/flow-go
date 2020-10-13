package topology

import (
	"fmt"
	"math"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

type TopicAwareTopology struct {
	RandPermTopology
	roleByTopic map[string]flow.RoleList // used to map between topics and roles subscribed to topics
}

func (t *TopicAwareTopology) Subset(idList flow.IdentityList, _ uint, topic string) (flow.IdentityList, error) {
	// extracts flow roles subscribed to topic.
	roles, ok := t.roleByTopic[topic]
	if !ok {
		return nil, fmt.Errorf("unknown topic with no subscribed roles: %s", topic)
	}

	// extracts node ids with specified roles.
	ids := idList.Filter(filter.HasRole(roles...))
	fanout := math.Ceil((float64)(len(ids)+1) / 2)
	randomSample, err := t.RandPermTopology.Subset(ids, uint(fanout), DummyTopic)

	if err != nil {
		return nil, fmt.Errorf("could not ")
	}
}
