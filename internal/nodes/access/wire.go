//+build wireinject

package access

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/nodes/access/config"
	"github.com/dapperlabs/bamboo-node/internal/nodes/ping"
	"github.com/dapperlabs/bamboo-node/internal/protocol/collect"
	"github.com/dapperlabs/bamboo-node/internal/protocol/observe"
	"github.com/dapperlabs/bamboo-node/internal/protocol/verify"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		config.New,
		ping.NewController,
		observe.NewController,
		collect.NewController,
		verify.NewController,
	)
	return &Server{}, nil
}
