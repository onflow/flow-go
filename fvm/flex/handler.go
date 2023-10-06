package flex

import (
	"bytes"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	env "github.com/onflow/flow-go/fvm/flex/environment"
	"github.com/onflow/flow-go/fvm/flex/models"
	"github.com/onflow/flow-go/fvm/flex/storage"
	"github.com/onflow/flow-go/model/flow"
)

type FlexContractHandler struct {
	db      *storage.Database
	backend models.Backend
}

var _ models.FlexContractHandler = &FlexContractHandler{}

func NewFlexContractHandler(backend models.Backend, flexAddress flow.Address) *FlexContractHandler {
	db, err := storage.NewDatabase(backend, flexAddress)
	handleError(err)
	return &FlexContractHandler{
		db:      db,
		backend: backend,
	}
}

// AllocateAddress allocates an address to be used by FOA resources
func (h FlexContractHandler) AllocateAddress() models.FlexAddress {
	env := h.getNewDefaultEnv()
	addr, err := env.AllocateAddress()
	handleError(err)
	return addr
}

// AccountByAddress returns the account for the given flex address,
// if isFOA is set, account is controlled by the FVM and FOA resources
func (h FlexContractHandler) AccountByAddress(addr models.FlexAddress, isFOA bool) models.FlexAccount {
	return newFlexAccount(h, addr, isFOA)
}

// LastExecutedBlock returns the last executed block
func (h FlexContractHandler) LastExecutedBlock() *models.FlexBlock {
	block, err := h.db.GetLatestBlock()
	handleError(err)
	return block
}

// Run runs an rlpencoded evm transaction, collect the evm fees under the provided coinbase
func (h FlexContractHandler) Run(rlpEncodedTx []byte, coinbase models.FlexAddress) bool {
	config := env.NewFlexConfig(
		env.WithCoinbase(coinbase.ToCommon()),
		env.WithBlockNumber(env.BlockNumberForEVMRules))
	env, err := env.NewEnvironment(config, h.db)
	handleError(err)

	// Decode transaction encoding
	tx := types.Transaction{}
	// TODO: update the max limit on the encoded size to a meaningful value
	err = tx.DecodeRLP(
		rlp.NewStream(
			bytes.NewReader(rlpEncodedTx),
			uint64(len(rlpEncodedTx))))
	handleError(err)

	// check tx gas limit
	// TODO: let caller set a limit as well
	gasLimit := tx.Gas()
	h.checkGasLimit(models.GasLimit(gasLimit))
	err = env.RunTransaction(&tx)
	h.meterGasUsage(env.Result.GasConsumed)

	if models.IsEVMExecutionError(err) {
		return false
	}

	handleError(err)
	// emit logs as events
	for _, log := range env.Result.Logs {
		h.EmitEvent(models.NewEVMLogEvent(log))
	}
	h.EmitLastExecutedBlockEvent()
	return true
}

func (h FlexContractHandler) checkGasLimit(limit models.GasLimit) {
	// check gas limit against what has been left on the transaction side
	if !h.backend.HasComputationCapacity(environment.ComputationKindEVMGasUsage, uint(limit)) {
		panic(models.ErrInsufficientComputation)
	}
}

func (h FlexContractHandler) meterGasUsage(usage uint64) {
	err := h.backend.MeterComputation(environment.ComputationKindEVMGasUsage, uint(usage))
	handleError(err)
}

func (h FlexContractHandler) getNewDefaultConfig() *env.Config {
	return env.NewFlexConfig(
		env.WithBlockNumber(env.BlockNumberForEVMRules))
}

func (h FlexContractHandler) getNewDefaultEnv() *env.Environment {
	env, err := env.NewEnvironment(h.getNewDefaultConfig(), h.db)
	handleError(err)
	return env
}

func handleError(err error) {
	if err != nil {
		if models.IsAFatalError(err) {
			// don't wrap it
			panic(err)
		}
		panic(errors.NewEVMError(err))
	}
}

func (h *FlexContractHandler) EmitEvent(event *models.Event) {
	// TODO add extra metering for encoding
	encoded, err := event.Payload.RLPEncode()
	handleError(err)
	h.backend.EmitFlowEvent(event.Etype, encoded)
}

func (h *FlexContractHandler) EmitLastExecutedBlockEvent() {
	// TODO: we should handle loading of blocks here and not inside db
	block, err := h.db.GetLatestBlock()
	handleError(err)
	h.EmitEvent(models.NewBlockExecutedEvent(block))
}

type flexAccount struct {
	isFOA   bool
	address models.FlexAddress
	fch     FlexContractHandler
}

// newFlexAccount creates a new flex account
func newFlexAccount(fch FlexContractHandler, addr models.FlexAddress, isFOA bool) *flexAccount {
	return &flexAccount{
		isFOA:   isFOA,
		fch:     fch,
		address: addr,
	}
}

// Address returns the flex address associated with the FOA account
func (f *flexAccount) Address() models.FlexAddress {
	return f.address
}

// Balance returns the balance of this foa
func (f *flexAccount) Balance() models.Balance {
	env := f.fch.getNewDefaultEnv()
	bl, err := env.Balance(f.address)
	handleError(err)
	balance, err := models.NewBalanceFromAttoFlow(bl)
	handleError(err)
	return balance
}

// Deposit deposits the token from the given vault into the Flex main vault
// and update the FOA balance with the new amount
func (f *flexAccount) Deposit(v *models.FLOWTokenVault) {
	env := f.getNewDefaultEnv()
	// TODO check gas limit and meter
	err := env.MintTo(v.Balance().ToAttoFlow(), f.address.ToCommon())
	f.fch.meterGasUsage(env.Result.GasConsumed)
	handleError(err)
	// emit event
	f.fch.EmitEvent(models.NewFlowTokenDepositEvent(f.address, v.Balance()))
	f.fch.EmitLastExecutedBlockEvent()
}

// Withdraw deducts the balance from the FOA account and
// withdraw and return flow token from the Flex main vault.
func (f *flexAccount) Withdraw(b models.Balance) *models.FLOWTokenVault {
	f.checkAuthorized()
	// TODO check gas limit and meter
	env := f.getNewDefaultEnv()
	err := env.WithdrawFrom(b.ToAttoFlow(), f.address.ToCommon())
	f.fch.meterGasUsage(env.Result.GasConsumed)
	handleError(err)
	// emit event
	f.fch.EmitEvent(models.NewFlowTokenWithdrawalEvent(f.address, b))
	f.fch.EmitLastExecutedBlockEvent()
	return models.NewFlowTokenVault(b)
}

// Deploy deploys a contract to the Flex environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the FOA accounts
func (f *flexAccount) Deploy(code models.Code, gaslimit models.GasLimit, balance models.Balance) models.FlexAddress {
	f.checkAuthorized()
	f.fch.checkGasLimit(gaslimit)
	env := f.getNewDefaultEnv()
	// TODO check gas limit against what has been left on the transaction side
	err := env.Deploy(f.address.ToCommon(), code, uint64(gaslimit), balance.ToAttoFlow())
	f.fch.meterGasUsage(env.Result.GasConsumed)
	handleError(err)
	f.fch.EmitLastExecutedBlockEvent()
	return models.FlexAddress(env.Result.DeployedContractAddress)
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
// contract data is not controlled by the FOA accounts
func (f *flexAccount) Call(to models.FlexAddress, data models.Data, gaslimit models.GasLimit, balance models.Balance) models.Data {
	f.checkAuthorized()
	f.fch.checkGasLimit(gaslimit)
	env := f.getNewDefaultEnv()
	err := env.Call(f.address.ToCommon(), to.ToCommon(), data, uint64(gaslimit), balance.ToAttoFlow())
	f.fch.meterGasUsage(env.Result.GasConsumed)
	handleError(err)
	f.fch.EmitLastExecutedBlockEvent()
	return env.Result.RetValue
}

func (f *flexAccount) getNewDefaultEnv() *env.Environment {
	return f.fch.getNewDefaultEnv()
}

func (f *flexAccount) checkAuthorized() {
	// check if account is authorized to to FOA related opeartions
	if !f.isFOA {
		handleError(models.ErrUnAuthroizedMethodCall)
	}
}
