//+build wireinject

package security

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/security/config"
	"github.com/dapperlabs/bamboo-node/internal/security/controllers"
	"github.com/dapperlabs/bamboo-node/internal/security/data"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		config.New,
		data.New,
		controllers.NewController,
	)
	return &Server{}, nil
}
