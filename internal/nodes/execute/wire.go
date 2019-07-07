//+build wireinject

package execute

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/nodes/execute/config"
	"github.com/dapperlabs/bamboo-node/internal/nodes/ping"
	"github.com/dapperlabs/bamboo-node/internal/protocol/execute"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		config.New,
		ping.NewController,
		execute.NewController,
	)
	return &Server{}, nil
}
