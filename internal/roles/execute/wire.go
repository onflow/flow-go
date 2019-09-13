//+build wireinject

package execute

import (
	"github.com/google/wire"

	"github.com/dapperlabs/flow-go/internal/roles/execute/config"
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
