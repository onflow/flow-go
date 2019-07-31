package collect

import (
	"github.com/dapperlabs/bamboo-node/internal/roles/collect/config"
	"github.com/dapperlabs/bamboo-node/pkg/data/keyvalue"
)

func NewDatabaseConnector(conf *config.Config) keyvalue.DBConnector {
	return keyvalue.NewpostgresDB(
		conf.PostgresAddr,
		conf.PostgresUser,
		conf.PostgresPassword,
		conf.PostgresDatabase,
	)
}
