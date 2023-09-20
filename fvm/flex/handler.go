package flex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/atree"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
)

type BaseFlexContractHandler struct {
	db *storage.Database
}

var _ models.FlexContractHandler = &BaseFlexContractHandler{}

func NewBaseFlexContractHandler(ledger atree.Ledger) *BaseFlexContractHandler {
	return &BaseFlexContractHandler{
		db: storage.NewDatabase(ledger),
	}
}

func (h BaseFlexContractHandler) NewFlowOwnedAccount() models.FlowOwnedAccount {
	panic("not implemented yet")
}

func (h BaseFlexContractHandler) LastExecutedBlock() models.FlexBlock {
	panic("not implemented yet")
}

func (h BaseFlexContractHandler) Run(tx []byte, coinbase models.FlexAddress) bool {
	config := env.NewFlexConfig(
		env.WithCoinbase(common.Address(coinbase)),
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.db)
	// TODO improve this
	if err != nil {
		panic(err)
	}
	env.RunTransaction(tx)
	return env.Result.Failed
}
