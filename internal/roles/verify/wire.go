//+build wireinject

package verify

import (
	"github.com/google/wire"

	"github.com/dapperlabs/bamboo-node/internal/roles/verify/config"
	"github.com/dapperlabs/bamboo-node/internal/roles/verify/processor"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		config.New,
		NewController,
		NewReceiptProcessorConfig,
		NewHasher,
		processor.NewReceiptProcessor,
		processor.NewEffectsProvider,
	)
	return &Server{}, nil
}
