//+build wireinject

package collect

import (
	"github.com/google/wire"
	"github.com/sirupsen/logrus"

	"github.com/dapperlabs/flow-go/internal/roles/collect/controller"
	"github.com/dapperlabs/flow-go/internal/roles/collect/storage"
	"github.com/dapperlabs/flow-go/internal/roles/collect/txpool"
)

// InitializeServer resolves all dependencies for dependency injection and returns the server object
func InitializeServer() (*Server, error) {
	wire.Build(
		NewServer,
		NewConfig,
		NewDatabaseConnector,
		storage.NewDatabaseStorage,
		txpool.New,
		logrus.New,
		controller.New,
	)
	return &Server{}, nil
}
