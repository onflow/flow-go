package migrations

import (
	coreContracts "github.com/onflow/flow-core-contracts/lib/go/contracts"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
)

func NewBurnerDeploymentMigration(
	chainID flow.ChainID,
	logger zerolog.Logger,
) RegistersMigration {
	address := BurnerAddressForChain(chainID)
	return NewDeploymentMigration(
		chainID,
		Contract{
			Name: "Burner",
			Code: coreContracts.Burner(),
		},
		address,
		map[flow.Address]struct{}{
			address: {},
		},
		logger,
	)
}
