package subscriptionmanager

import (
	"github.com/onflow/flow-go/network"
)

type SubscriptionManager interface {
	// Register registers an engine on the channel ID into the subscription manager.
	Register(channelID string, engine network.Engine) error

	// Unregister removes the engine associated with a channel ID
	Unregister(channelID string) error

	// GetEngine returns engine associated with a channel ID.
	GetEngine(channelID string) (network.Engine, error)

	// GetChannelID returns all the channel IDs registered in this subscription manager.
	GetChannelIDs() []string
}
