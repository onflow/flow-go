package flex

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/onflow/atree"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/storage"
)

type FlexAddress common.Address

// TODO add utility for conversion

func NewFlexAddressFromString(str string) FlexAddress {
	return FlexAddress(common.BytesToAddress([]byte(str)))
}

type FlexContractHandler interface {
	Run(bytes []byte, coinbase FlexAddress) bool
}

type BaseFlexContractHandler struct {
	db *storage.Database
}

func NewBaseFlexContractHandler(ledger atree.Ledger) *BaseFlexContractHandler {
	return &BaseFlexContractHandler{
		db: storage.NewDatabase(ledger),
	}
}

func (h BaseFlexContractHandler) Run(tx []byte, coinbase FlexAddress) bool {
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
