package topology

import "github.com/onflow/flow-go/model/flow"

// Todo: deprecate non-topic-aware Topology implementations.
// DummyTopic represents an empty topic name that is used to pass to Subset method of
// Topology implementations that do not support topics.
const DummyTopic = ""

// Topology provides a subset of nodes which a given node should directly connect to for 1-k messaging
type Topology interface {
	// Subset returns a random subset of the identity list that is passed
	Subset(idList flow.IdentityList, fanout uint, topic string) (flow.IdentityList, error)
}
