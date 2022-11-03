package corruptlibp2p

import (
	corruptpubsub "github.com/yhassanzadeh13/go-libp2p-pubsub"
)

// getGossipSubParams is a small test function to test that can access forked pubsub module
func getGossipSubParams() corruptpubsub.GossipSubParams {
	defaultParams := corruptpubsub.DefaultGossipSubParams()
	return defaultParams
}
