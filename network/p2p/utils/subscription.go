package p2putils

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/network/channels"
)

func AllowedSubscription(role flow.Role, topic string) bool {
	channel, ok := channels.ChannelFromTopic(channels.Topic(topic))
	if !ok {
		return false
	}

	if !role.Valid() {
		// TODO: eventually we should have block proposals relayed on a separate
		// channel on the public network. For now, we need to make sure that
		// full observer nodes can subscribe to the block proposal channel.
		return append(channels.PublicChannels(), channels.ReceiveBlocks).Contains(channel)
	} else {
		if roles, ok := channels.RolesByChannel(channel); ok {
			return roles.Contains(role)
		}

		return false
	}
}
