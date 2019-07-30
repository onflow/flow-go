//+build wireinject

package collect

import (
	"github.com/google/wire"
	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/bamboo-node/internal/roles/collect/config"
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/controller"
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/data"
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/txpool"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		config.New,
		data.New,
		txpool.New,
		logrus.New,
		controller.New,
	)
	return &Server{}, nil
}
