//+build wireinject

package security

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/nodes/ping"
	"github.com/dapperlabs/bamboo-node/internal/nodes/security/config"
	"github.com/dapperlabs/bamboo-node/internal/protocol/consensus"
	"github.com/dapperlabs/bamboo-node/internal/protocol/seal"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		config.New,
		ping.NewController,
		consensus.NewController,
		seal.NewController,
	)
	return &Server{}, nil
}
