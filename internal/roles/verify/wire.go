//+build wireinject

package verify

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/services/verify/config"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		config.New,
		NewController,
	)
	return &Server{}, nil
}
