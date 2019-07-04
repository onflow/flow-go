//+build wireinject

package execute

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/nodes/execute/config"
	"github.com/dapperlabs/bamboo-node/internal/nodes/execute/controllers"
	"github.com/dapperlabs/bamboo-node/internal/nodes/execute/data"
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
