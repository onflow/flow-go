package validation

import "github.com/onflow/flow-go/network/channels"

type duplicateTopicTracker map[channels.Topic]struct{}

func (d duplicateTopicTracker) set(topic channels.Topic) {
	d[topic] = struct{}{}
}

func (d duplicateTopicTracker) isDuplicate(topic channels.Topic) bool {
	_, ok := d[topic]
	return ok
}
