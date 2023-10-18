package flex

import (
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/flex/models"
)

// FlexContractHandler is responsible for triggering calls to emulator, metering,
// event emission and updating the block
//
// TODO and Warning: currently database keeps a copy of roothash, and if after
// commiting the changes by the evm we want to revert in this code we need to reset that
// or we should always do all the checks and return before calling the emulator,
// after that should be only event emissions and computation usage updates.
// thats another reason we first check the computation limit before using.
// in the future we might benefit from a view style of access to db passed as
// a param to the emulator.
type FlexContractHandler struct {
	blockchain        models.BlockChain
	backend           models.Backend
	emulator          models.Emulator
	lastExecutedBlock *models.FlexBlock
	uuidIndex         uint64
	totalSupply       uint64
}

var _ models.FlexContractHandler = &FlexContractHandler{}

func NewFlexContractHandler(
	blockchain models.BlockChain,
	backend models.Backend,
	emulator models.Emulator,
) *FlexContractHandler {
	lastExecutedBlock, err := blockchain.LatestBlock()
	// fatal error
	if err != nil {
		panic(err)
	}

	return &FlexContractHandler{
		blockchain:        blockchain,
		backend:           backend,
		emulator:          emulator,
		lastExecutedBlock: lastExecutedBlock,
		uuidIndex:         lastExecutedBlock.UUIDIndex,
		totalSupply:       lastExecutedBlock.TotalSupply,
	}
}

// AllocateAddress allocates an address to be used by FOA resources
func (h *FlexContractHandler) AllocateAddress() models.FlexAddress {
	target := models.FlexAddress{}
	// first 12 bytes would be zero
	// the next 8 bytes would be incremented of uuid
	binary.BigEndian.PutUint64(target[12:], h.lastExecutedBlock.UUIDIndex)
	h.uuidIndex++

	// TODO: if account exist try some new number
	// if fe.State.Exist(target.ToCommon()) {
	// }

	h.updateLastExecutedBlock(h.lastExecutedBlock.StateRoot, types.EmptyRootHash)
	return target
}

// AccountByAddress returns the account for the given flex address,
// if isFOA is set, account is controlled by the FVM and FOA resources
func (h FlexContractHandler) AccountByAddress(addr models.FlexAddress, isFOA bool) models.FlexAccount {
	return newFlexAccount(h, addr, isFOA)
}

// LastExecutedBlock returns the last executed block
func (h FlexContractHandler) LastExecutedBlock() *models.FlexBlock {
	block, err := h.blockchain.LatestBlock()
	handleError(err)
	return block
}

func (h *FlexContractHandler) updateLastExecutedBlock(stateRoot, eventRoot common.Hash) {
	h.lastExecutedBlock = models.NewFlexBlock(
		h.lastExecutedBlock.Height+1,
		h.uuidIndex,
		h.totalSupply,
		stateRoot,
		eventRoot,
	)

	err := h.blockchain.AppendBlock(h.lastExecutedBlock)
	handleError(err)
}

// Run runs an rlpencoded evm transaction, collect the evm fees under the provided coinbase
func (h FlexContractHandler) Run(rlpEncodedTx []byte, coinbase models.FlexAddress) bool {
	// Decode transaction encoding
	tx := types.Transaction{}
	// TODO: update the max limit on the encoded size to a meaningful value
	err := tx.DecodeRLP(
		rlp.NewStream(
			bytes.NewReader(rlpEncodedTx),
			uint64(len(rlpEncodedTx))))
	handleError(err)

	// check tx gas limit
	gasLimit := tx.Gas()
	h.checkGasLimit(models.GasLimit(gasLimit))

	ctx := h.getBlockContext()
	ctx.GasFeeCollector = coinbase

	blk, err := h.emulator.NewBlock(ctx)
	handleError(err)

	res, err := blk.RunTransaction(&tx)
	h.meterGasUsage(res)

	failed := false
	if err != nil {
		// if error is fatal panic here
		if models.IsAFatalError(err) {
			// don't wrap it
			panic(err)
		}
		err = errors.NewEVMError(err)
		failed = true
	}
	if res == nil {
		// fatal error
		panic("empty result is retuned by emulator")
	}

	res.Failed = failed

	h.EmitEvent(models.NewTransactionExecutedEvent(h.lastExecutedBlock.Height+1, res))
	h.EmitLastExecutedBlockEvent()
	h.updateLastExecutedBlock(res.StateRootHash, types.EmptyRootHash)
	return !failed
}

func (h FlexContractHandler) checkGasLimit(limit models.GasLimit) {
	// check gas limit against what has been left on the transaction side
	if !h.backend.HasComputationCapacity(environment.ComputationKindEVMGasUsage, uint(limit)) {
		handleError(models.ErrInsufficientComputation)
	}
}

func (h FlexContractHandler) meterGasUsage(res *models.Result) {
	if res != nil {
		err := h.backend.MeterComputation(environment.ComputationKindEVMGasUsage, uint(res.GasConsumed))
		handleError(err)
	}
}

func (h *FlexContractHandler) EmitEvent(event *models.Event) {
	// TODO add extra metering for rlp encoding
	encoded, err := event.Payload.Encode()
	handleError(err)
	h.backend.EmitFlowEvent(event.Etype, encoded)
}

func (h *FlexContractHandler) EmitLastExecutedBlockEvent() {
	block, err := h.blockchain.LatestBlock()
	handleError(err)
	h.EmitEvent(models.NewBlockExecutedEvent(block))
}

func (h *FlexContractHandler) getBlockContext() models.BlockContext {
	return models.BlockContext{
		BlockNumber:            h.lastExecutedBlock.Height + 1,
		DirectCallBaseGasUsage: models.DefaultDirectCallBaseGasUsage,
	}
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
	ctx := f.fch.getBlockContext()

	blk, err := f.fch.emulator.NewBlockView(ctx)
	handleError(err)

	bl, err := blk.BalanceOf(f.address)
	handleError(err)

	balance, err := models.NewBalanceFromAttoFlow(bl)
	handleError(err)
	return balance
}

// Deposit deposits the token from the given vault into the Flex main vault
// and update the FOA balance with the new amount
func (f *flexAccount) Deposit(v *models.FLOWTokenVault) {
	cfg := f.fch.getBlockContext()
	f.fch.checkGasLimit(models.GasLimit(cfg.DirectCallBaseGasUsage))

	blk, err := f.fch.emulator.NewBlock(cfg)
	handleError(err)

	res, err := blk.MintTo(f.address, v.Balance().ToAttoFlow())
	f.fch.meterGasUsage(res)
	handleError(err)
	// emit event
	f.fch.EmitEvent(models.NewFlowTokenDepositEvent(f.address, v.Balance()))
	f.fch.EmitLastExecutedBlockEvent()
	f.fch.totalSupply += v.Balance().ToAttoFlow().Uint64()
	f.fch.updateLastExecutedBlock(res.StateRootHash, types.EmptyRootHash)
}

// Withdraw deducts the balance from the FOA account and
// withdraw and return flow token from the Flex main vault.
func (f *flexAccount) Withdraw(b models.Balance) *models.FLOWTokenVault {
	f.checkAuthorized()

	// check balance of flex vault
	if b.ToAttoFlow().Uint64() > f.fch.totalSupply {
		handleError(models.ErrInsufficientTotalSupply)
	}

	cfg := f.fch.getBlockContext()
	f.fch.checkGasLimit(models.GasLimit(cfg.DirectCallBaseGasUsage))

	blk, err := f.fch.emulator.NewBlock(cfg)
	handleError(err)

	res, err := blk.WithdrawFrom(f.address, b.ToAttoFlow())
	f.fch.meterGasUsage(res)
	handleError(err)

	// emit event
	f.fch.EmitEvent(models.NewFlowTokenWithdrawalEvent(f.address, b))
	f.fch.EmitLastExecutedBlockEvent()
	f.fch.totalSupply -= b.ToAttoFlow().Uint64()
	f.fch.updateLastExecutedBlock(res.StateRootHash, types.EmptyRootHash)
	return models.NewFlowTokenVault(b)
}

// Transfer transfers tokens between accounts
func (f *flexAccount) Transfer(to models.FlexAddress, balance models.Balance) {
	f.checkAuthorized()

	cfg := f.fch.getBlockContext()
	f.fch.checkGasLimit(models.GasLimit(cfg.DirectCallBaseGasUsage))

	blk, err := f.fch.emulator.NewBlock(cfg)
	handleError(err)

	res, err := blk.Transfer(f.address, to, balance.ToAttoFlow())
	f.fch.meterGasUsage(res)
	handleError(err)

	f.fch.EmitLastExecutedBlockEvent()
	f.fch.updateLastExecutedBlock(res.StateRootHash, types.EmptyRootHash)
}

// Deploy deploys a contract to the Flex environment
// the new deployed contract would be at the returned address and
// the contract data is not controlled by the FOA accounts
func (f *flexAccount) Deploy(code models.Code, gaslimit models.GasLimit, balance models.Balance) models.FlexAddress {
	f.checkAuthorized()
	f.fch.checkGasLimit(gaslimit)

	blk, err := f.fch.emulator.NewBlock(f.fch.getBlockContext())
	handleError(err)

	res, err := blk.Deploy(f.address, code, uint64(gaslimit), balance.ToAttoFlow())
	f.fch.meterGasUsage(res)
	handleError(err)
	f.fch.EmitLastExecutedBlockEvent()
	f.fch.updateLastExecutedBlock(res.StateRootHash, types.EmptyRootHash)
	return models.FlexAddress(res.DeployedContractAddress)
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
// contract data is not controlled by the FOA accounts
func (f *flexAccount) Call(to models.FlexAddress, data models.Data, gaslimit models.GasLimit, balance models.Balance) models.Data {
	f.checkAuthorized()
	f.fch.checkGasLimit(gaslimit)

	blk, err := f.fch.emulator.NewBlock(f.fch.getBlockContext())
	handleError(err)

	res, err := blk.Call(f.address, to, data, uint64(gaslimit), balance.ToAttoFlow())
	f.fch.meterGasUsage(res)
	handleError(err)
	f.fch.EmitLastExecutedBlockEvent()
	// TODO: update this to calculate receipt hash
	f.fch.updateLastExecutedBlock(res.StateRootHash, types.EmptyRootHash)
	return res.ReturnedValue
}

func (f *flexAccount) checkAuthorized() {
	// check if account is authorized to to FOA related opeartions
	if !f.isFOA {
		handleError(models.ErrUnAuthroizedMethodCall)
	}
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
