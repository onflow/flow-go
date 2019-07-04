//+build wireinject

package access

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/nodes/access/config"
	"github.com/dapperlabs/bamboo-node/internal/nodes/access/controllers"
	"github.com/dapperlabs/bamboo-node/internal/nodes/access/data"
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
