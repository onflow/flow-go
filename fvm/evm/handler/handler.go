package handler

import (
	"fmt"
	"math/big"

	gethCommon "github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/onflow/cadence/common"
	"go.opentelemetry.io/otel/attribute"

	"github.com/onflow/flow-go/fvm/environment"
	fvmErrors "github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/handler/coa"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/trace"
)

// ContractHandler is responsible for triggering calls to emulator, metering,
// event emission and updating the block
type ContractHandler struct {
	flowChainID          flow.ChainID
	evmContractAddress   flow.Address
	flowTokenAddress     common.Address
	blockStore           types.BlockStore
	addressAllocator     types.AddressAllocator
	backend              types.Backend
	emulator             types.Emulator
	precompiledContracts []types.PrecompiledContract
}

var _ types.ContractHandler = &ContractHandler{}

// NewContractHandler constructs a new ContractHandler
func NewContractHandler(
	flowChainID flow.ChainID,
	evmContractAddress flow.Address,
	flowTokenAddress common.Address,
	randomBeaconAddress flow.Address,
	blockStore types.BlockStore,
	addressAllocator types.AddressAllocator,
	backend types.Backend,
	emulator types.Emulator,
) *ContractHandler {
	return &ContractHandler{
		flowChainID:        flowChainID,
		evmContractAddress: evmContractAddress,
		flowTokenAddress:   flowTokenAddress,
		blockStore:         blockStore,
		addressAllocator:   addressAllocator,
		backend:            backend,
		emulator:           emulator,
		precompiledContracts: preparePrecompiledContracts(
			evmContractAddress,
			randomBeaconAddress,
			addressAllocator,
			backend,
		),
	}
}

// FlowTokenAddress returns the address where the FlowToken contract is deployed
func (h *ContractHandler) FlowTokenAddress() common.Address {
	return h.flowTokenAddress
}

// EVMContractAddress returns the address where EVM contract is deployed
func (h *ContractHandler) EVMContractAddress() common.Address {
	return common.Address(h.evmContractAddress)
}

// DeployCOA deploys a cadence-owned-account and returns the address
func (h *ContractHandler) DeployCOA(uuid uint64) types.Address {
	// capture open tracing traces
	defer h.backend.StartChildSpan(trace.FVMEVMDeployCOA).End()

	res, err := h.deployCOA(uuid)
	panicOnErrorOrInvalidOrFailedState(res, err)

	return *res.DeployedContractAddress
}

func (h *ContractHandler) deployCOA(uuid uint64) (*types.Result, error) {
	// step 1 - check enough computation is available
	gaslimit := types.GasLimit(coa.ContractDeploymentRequiredGas)
	err := h.checkGasLimit(gaslimit)
	if err != nil {
		return nil, err
	}

	// step 2 - allocate a new address for the COA
	target := h.addressAllocator.AllocateCOAAddress(uuid)

	// step 3 - create a COA deployment call
	factory := h.addressAllocator.COAFactoryAddress()
	factoryAccount := h.AccountByAddress(factory, false)
	factoryNonce := factoryAccount.Nonce()
	call := types.NewDeployCallWithTargetAddress(
		factory,
		target,
		coa.ContractBytes,
		uint64(gaslimit),
		new(big.Int),
		factoryNonce,
	)

	// step 4 - execute the call
	res, err := h.executeAndHandleCall(call, nil, false)
	if err != nil {
		return nil, err
	}

	// step 5 - if successful COA metrics
	h.backend.SetNumberOfDeployedCOAs(factoryNonce)
	return res, nil
}

// AccountByAddress returns the account for the given address,
// if isAuthorized is set, account is controlled by the FVM (COAs)
func (h *ContractHandler) AccountByAddress(addr types.Address, isAuthorized bool) types.Account {
	return newAccount(h, addr, isAuthorized)
}

// LastExecutedBlock returns the last executed block
func (h *ContractHandler) LastExecutedBlock() *types.Block {
	block, err := h.blockStore.LatestBlock()
	panicOnError(err)
	return block
}

// RunOrPanic runs an rlp-encoded evm transaction and
func (h *ContractHandler) RunOrPanic(rlpEncodedTx []byte, gasFeeCollector types.Address) {
	// capture open tracing span
	defer h.backend.StartChildSpan(trace.FVMEVMRun).End()

	h.runWithGasFeeRefund(gasFeeCollector, func() {
		res, err := h.run(rlpEncodedTx)
		panicOnErrorOrInvalidOrFailedState(res, err)
	})
}

// Run tries to run an rlp-encoded evm transaction
// collects the gas fees and pay it to the gasFeeCollector address provided.
func (h *ContractHandler) Run(rlpEncodedTx []byte, gasFeeCollector types.Address) *types.ResultSummary {
	// capture open tracing span
	defer h.backend.StartChildSpan(trace.FVMEVMRun).End()

	var res *types.Result
	var err error
	h.runWithGasFeeRefund(gasFeeCollector, func() {
		// run transaction
		res, err = h.run(rlpEncodedTx)
		panicOnError(err)

	})
	// return the result summary
	return res.ResultSummary()
}

// runWithGasFeeRefund runs a method and transfers the balance changes of the
// coinbase address to the provided gas fee collector
func (h *ContractHandler) runWithGasFeeRefund(gasFeeCollector types.Address, f func()) {
	// capture coinbase init balance
	cb := h.AccountByAddress(types.CoinbaseAddress, true)
	initCoinbaseBalance := cb.Balance()
	f()
	// transfer the gas fees collected to the gas fee collector address
	afterBalance := cb.Balance()
	diff := new(big.Int).Sub(afterBalance, initCoinbaseBalance)
	if diff.Sign() > 0 {
		cb.Transfer(gasFeeCollector, diff)
	}
	if diff.Sign() < 0 { // this should never happen but in case
		panic(fvmErrors.NewEVMError(fmt.Errorf("negative balance change on coinbase")))
	}
}

// BatchRun tries to run batch of rlp-encoded transactions
// collects the gas fees and pay it to the coinbase address provided.
// All transactions provided in the batch are included in a single block,
// except for invalid transactions
func (h *ContractHandler) BatchRun(rlpEncodedTxs [][]byte, gasFeeCollector types.Address) []*types.ResultSummary {
	// capture open tracing
	span := h.backend.StartChildSpan(trace.FVMEVMBatchRun)
	span.SetAttributes(attribute.Int("tx_counts", len(rlpEncodedTxs)))
	defer span.End()

	var results []*types.Result
	var err error
	h.runWithGasFeeRefund(gasFeeCollector, func() {
		// batch run transactions and panic if any error
		results, err = h.batchRun(rlpEncodedTxs)
		panicOnError(err)
	})

	// convert results into result summaries
	resSummaries := make([]*types.ResultSummary, len(results))
	for i, r := range results {
		resSummaries[i] = r.ResultSummary()
	}
	return resSummaries
}

func (h *ContractHandler) batchRun(rlpEncodedTxs [][]byte) ([]*types.Result, error) {
	// step 1 - transaction decoding and check that enough evm gas is available in the FVM transaction

	// remainingGasLimit is the remaining EVM gas available in hte FVM transaction
	remainingGasLimit := h.backend.ComputationRemaining(environment.ComputationKindEVMGasUsage)
	batchLen := len(rlpEncodedTxs)
	txs := make([]*gethTypes.Transaction, batchLen)
	for i, rlpEncodedTx := range rlpEncodedTxs {
		tx, err := h.decodeTransaction(rlpEncodedTx)
		// if any tx fails decoding revert the batch
		if err != nil {
			return nil, err
		}

		txs[i] = tx

		// step 2 - check if enough computation is available
		txGasLimit := tx.Gas()
		if remainingGasLimit < txGasLimit {
			return nil, types.ErrInsufficientComputation
		}
		remainingGasLimit -= txGasLimit
	}

	// step 3 - prepare block context
	bp, err := h.getBlockProposal()
	if err != nil {
		return nil, err
	}

	ctx, err := h.getBlockContext(bp)
	if err != nil {
		return nil, err
	}

	// step 4 - create a block view
	blk, err := h.emulator.NewBlockView(ctx)
	if err != nil {
		return nil, err
	}

	var res []*types.Result
	// step 5 - batch run transactions
	h.backend.RunWithMeteringDisabled(
		func() {
			res, err = blk.BatchRunTransactions(txs)
		},
	)

	if err != nil {
		return nil, err
	}
	if len(res) == 0 { // safety check for result
		return nil, types.ErrUnexpectedEmptyResult
	}

	var hasAtLeastOneValid bool
	// step 6 - meter all the transaction gas usage
	// and append hashes to the block
	for _, r := range res {
		// meter gas anyway (even for invalid or failed states)
		err = h.meterGasUsage(r)
		if err != nil {
			return nil, err
		}

		// include it in a block only if valid (not invalid)
		if !r.Invalid() {
			hasAtLeastOneValid = true
		}
	}

	// step 7 - if there were no valid transactions
	// skip the rest of steps
	if !hasAtLeastOneValid {
		return res, nil
	}

	// for valid transactions
	for i, r := range res {
		if r.Invalid() { // don't emit events for invalid tx
			continue
		}

		// step 8 - update block proposal
		bp.AppendTransaction(r)

		// step 9 - emit transaction event
		err = h.emitEvent(events.NewTransactionEvent(
			r,
			rlpEncodedTxs[i],
			bp.Height,
		))
		if err != nil {
			return nil, err
		}

		// step 10 - report metrics
		h.backend.EVMTransactionExecuted(
			r.GasConsumed,
			false,
			r.Failed(),
		)
	}

	// update the block proposal
	err = h.blockStore.UpdateBlockProposal(bp)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// CommitBlockProposal commits the block proposal
// and add a new block to the EVM blockchain
func (h *ContractHandler) CommitBlockProposal() {
	panicOnError(h.commitBlockProposal())
}

func (h *ContractHandler) commitBlockProposal() error {
	// load latest block proposal
	bp, err := h.blockStore.BlockProposal()
	if err != nil {
		return err
	}

	// commit the proposal
	err = h.blockStore.CommitBlockProposal(bp)
	if err != nil {
		return err
	}

	// emit block executed event
	err = h.emitEvent(events.NewBlockEvent(&bp.Block))
	if err != nil {
		return err
	}

	// report metrics
	h.backend.EVMBlockExecuted(
		len(bp.TxHashes),
		bp.TotalGasUsed,
		types.UnsafeCastOfBalanceToFloat64(bp.TotalSupply),
	)

	// log evm block commitment
	logger := h.backend.Logger()
	logger.Info().
		Uint64("evm_height", bp.Height).
		Int("tx_count", len(bp.TxHashes)).
		Uint64("total_gas_used", bp.TotalGasUsed).
		Uint64("total_supply", bp.TotalSupply.Uint64()).
		Msg("EVM Block Committed")

	return nil
}

func (h *ContractHandler) run(rlpEncodedTx []byte) (*types.Result, error) {
	// step 1 - transaction decoding
	tx, err := h.decodeTransaction(rlpEncodedTx)
	if err != nil {
		return nil, err
	}

	// step 2 - check if enough computation is available
	err = h.checkGasLimit(types.GasLimit(tx.Gas()))
	if err != nil {
		return nil, err
	}

	// step 3 - prepare block context
	// load block proposal
	bp, err := h.getBlockProposal()
	if err != nil {
		return nil, err
	}

	ctx, err := h.getBlockContext(bp)
	if err != nil {
		return nil, err
	}

	// step 4 - create a block view
	blk, err := h.emulator.NewBlockView(ctx)
	if err != nil {
		return nil, err
	}

	// step 5 - run transaction
	var res *types.Result
	h.backend.RunWithMeteringDisabled(
		func() {
			res, err = blk.RunTransaction(tx)
		})
	if err != nil {
		return nil, err
	}
	if res == nil { // safety check for result
		return nil, types.ErrUnexpectedEmptyResult
	}

	// step 6 - meter gas anyway (even for invalid or failed states)
	err = h.meterGasUsage(res)
	if err != nil {
		return nil, err
	}

	// step 7 - skip the rest if is invalid tx
	if res.Invalid() {
		return res, nil
	}

	// step 8 - update the block proposal
	bp.AppendTransaction(res)
	err = h.blockStore.UpdateBlockProposal(bp)
	if err != nil {
		return nil, err
	}

	// step 9 - emit transaction event
	err = h.emitEvent(
		events.NewTransactionEvent(res, rlpEncodedTx, bp.Height),
	)

	if err != nil {
		return nil, err
	}

	// step 10 - report metrics
	h.backend.EVMTransactionExecuted(
		res.GasConsumed,
		false,
		res.Failed(),
	)

	return res, nil
}

// DryRun simulates execution of the provided RLP-encoded and unsigned transaction.
func (h *ContractHandler) DryRun(
	rlpEncodedTx []byte,
	from types.Address,
) *types.ResultSummary {
	defer h.backend.StartChildSpan(trace.FVMEVMDryRun).End()

	res, err := h.dryRun(rlpEncodedTx, from)
	panicOnError(err)

	return res.ResultSummary()
}

func (h *ContractHandler) dryRun(
	rlpEncodedTx []byte,
	from types.Address,
) (*types.Result, error) {
	// step 1 - transaction decoding
	err := h.backend.MeterComputation(
		common.ComputationUsage{
			Kind:      environment.ComputationKindRLPDecoding,
			Intensity: uint64(len(rlpEncodedTx)),
		},
	)
	if err != nil {
		return nil, err
	}

	tx := gethTypes.Transaction{}
	err = tx.UnmarshalBinary(rlpEncodedTx)
	if err != nil {
		return nil, err
	}

	return h.dryRunTx(&tx, from)
}

func (h *ContractHandler) dryRunTx(
	tx *gethTypes.Transaction,
	from types.Address,
) (*types.Result, error) {
	// check if enough computation is available
	err := h.checkGasLimit(types.GasLimit(tx.Gas()))
	if err != nil {
		return nil, err
	}

	bp, err := h.getBlockProposal()
	if err != nil {
		return nil, err
	}
	ctx, err := h.getBlockContext(bp)
	if err != nil {
		return nil, err
	}

	blk, err := h.emulator.NewBlockView(ctx)
	if err != nil {
		return nil, err
	}

	var res *types.Result
	// just like with EVM.run / EVM.batchRun / COA.call, we disable metering
	// so we can fully meter the gas usage in the next step, even in case
	// of unhandled errors/exceptions.
	h.backend.RunWithMeteringDisabled(func() {
		res, err = blk.DryRunTransaction(tx, from.ToCommon())
	})
	if err != nil {
		return nil, err
	}
	if res == nil { // safety check for result
		return nil, types.ErrUnexpectedEmptyResult
	}

	// gas meter even invalid or failed status
	if err = h.meterGasUsage(res); err != nil {
		return nil, err
	}

	return res, nil
}

// DryRunWithTxData simulates execution of the provided transaction data.
// The from address is required since the transaction is unsigned.
// The function should not have any persisted changes made to the state.
func (h *ContractHandler) DryRunWithTxData(
	txData gethTypes.TxData,
	from types.Address,
) *types.ResultSummary {
	if txData == nil {
		panicOnError(types.ErrUnexpectedEmptyTransactionData)
	}

	defer h.backend.StartChildSpan(trace.FVMEVMDryRun).End()

	tx := gethTypes.NewTx(txData)

	res, err := h.dryRunTx(tx, from)
	panicOnError(err)

	return res.ResultSummary()
}

// checkGasLimit checks if enough computation is left in the environment
// before attempting executing a evm operation
func (h *ContractHandler) checkGasLimit(limit types.GasLimit) error {
	// check gas limit against what has been left on the transaction side
	usage := common.ComputationUsage{
		Kind:      environment.ComputationKindEVMGasUsage,
		Intensity: uint64(limit),
	}
	if !h.backend.ComputationAvailable(usage) {
		return types.ErrInsufficientComputation
	}
	return nil
}

// decodeTransaction decodes RLP encoded transaction payload and meters the resources used.
func (h *ContractHandler) decodeTransaction(encodedTx []byte) (*gethTypes.Transaction, error) {
	usage := common.ComputationUsage{
		Kind:      environment.ComputationKindRLPDecoding,
		Intensity: uint64(len(encodedTx)),
	}
	err := h.backend.MeterComputation(usage)
	if err != nil {
		return nil, err
	}

	tx := gethTypes.Transaction{}
	if err := tx.UnmarshalBinary(encodedTx); err != nil {
		return nil, err
	}

	return &tx, nil
}

func (h *ContractHandler) meterGasUsage(res *types.Result) error {
	usage := common.ComputationUsage{
		Kind:      environment.ComputationKindEVMGasUsage,
		Intensity: res.GasConsumed,
	}
	return h.backend.MeterComputation(usage)
}

func (h *ContractHandler) emitEvent(event *events.Event) error {
	ev, err := event.Payload.ToCadence(h.flowChainID)
	if err != nil {
		return err
	}
	return h.backend.EmitEvent(ev)
}

func (h *ContractHandler) getBlockContext(bp *types.BlockProposal) (
	types.BlockContext,
	error,
) {
	return types.BlockContext{
		ChainID:                types.EVMChainIDFromFlowChainID(h.flowChainID),
		BlockNumber:            bp.Height,
		BlockTimestamp:         bp.Timestamp,
		DirectCallBaseGasUsage: types.DefaultDirectCallBaseGasUsage,
		GetHashFunc: func(n uint64) gethCommon.Hash {
			hash, err := h.blockStore.BlockHash(n)
			panicOnError(err) // we have to handle it here given we can't continue with it even in try case
			return hash
		},
		ExtraPrecompiledContracts: h.precompiledContracts,
		Random:                    bp.PrevRandao,
		TxCountSoFar:              uint(len(bp.TxHashes)),
		TotalGasUsedSoFar:         bp.TotalGasUsed,
		GasFeeCollector:           types.CoinbaseAddress,
	}, nil
}

func (h *ContractHandler) getBlockProposal() (*types.BlockProposal, error) {
	return h.blockStore.BlockProposal()
}

func (h *ContractHandler) executeAndHandleCall(
	call *types.DirectCall,
	totalSupplyDiff *big.Int,
	deductSupplyDiff bool,
) (*types.Result, error) {
	// step 1 - check enough computation is available
	if err := h.checkGasLimit(types.GasLimit(call.GasLimit)); err != nil {
		return nil, err
	}

	// step 2 - prepare the block context
	// load the block proposal
	bp, err := h.getBlockProposal()
	if err != nil {
		return nil, err
	}
	ctx, err := h.getBlockContext(bp)
	if err != nil {
		return nil, err
	}

	// step 3 - create block view
	blk, err := h.emulator.NewBlockView(ctx)
	if err != nil {
		return nil, err
	}

	// step 4 - run direct call
	var res *types.Result
	h.backend.RunWithMeteringDisabled(
		func() {
			res, err = blk.DirectCall(call)
		})
	// check backend errors first
	if err != nil {
		return nil, err
	}
	if res == nil { // safety check for result
		return nil, types.ErrUnexpectedEmptyResult
	}

	// step 5 - gas meter even invalid or failed status
	err = h.meterGasUsage(res)
	if err != nil {
		return nil, err
	}

	// step 6 - if is invalid skip the rest of states
	if res.Invalid() {
		return res, nil
	}

	// step 7 - update block proposal
	// append transaction to the block proposal
	bp.AppendTransaction(res)

	// update total supply if applicable
	if res.Successful() && totalSupplyDiff != nil {
		if deductSupplyDiff {
			bp.TotalSupply = new(big.Int).Sub(bp.TotalSupply, totalSupplyDiff)
			if bp.TotalSupply.Sign() < 0 {
				return nil, types.ErrInsufficientTotalSupply
			}
		} else {
			bp.TotalSupply = new(big.Int).Add(bp.TotalSupply, totalSupplyDiff)
		}
	}

	// update the block proposal
	err = h.blockStore.UpdateBlockProposal(bp)
	if err != nil {
		return nil, err
	}

	// step 8 - emit transaction event
	encoded, err := call.Encode()
	if err != nil {
		return nil, err
	}
	err = h.emitEvent(
		events.NewTransactionEvent(res, encoded, bp.Height),
	)
	if err != nil {
		return nil, err
	}

	// step 9 - report metrics
	h.backend.EVMTransactionExecuted(
		res.GasConsumed,
		true,
		res.Failed(),
	)

	return res, nil
}

func (h *ContractHandler) GenerateResourceUUID() uint64 {
	uuid, err := h.backend.GenerateUUID()
	panicOnError(err)
	return uuid
}

type Account struct {
	isAuthorized bool
	address      types.Address
	fch          *ContractHandler
}

// newAccount creates a new evm account
func newAccount(fch *ContractHandler, addr types.Address, isAuthorized bool) *Account {
	return &Account{
		isAuthorized: isAuthorized,
		fch:          fch,
		address:      addr,
	}
}

// Address returns the address associated with the account
func (a *Account) Address() types.Address {
	return a.address
}

// Nonce returns the nonce of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already translates into computation
func (a *Account) Nonce() uint64 {
	nonce, err := a.nonce()
	panicOnError(err)
	return nonce
}

func (a *Account) nonce() (uint64, error) {
	blk, err := a.fch.emulator.NewReadOnlyBlockView()
	if err != nil {
		return 0, err
	}

	return blk.NonceOf(a.address)
}

// Balance returns the balance of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already translates into computation
func (a *Account) Balance() types.Balance {
	bal, err := a.balance()
	panicOnError(err)
	return bal
}

func (a *Account) balance() (types.Balance, error) {
	blk, err := a.fch.emulator.NewReadOnlyBlockView()
	if err != nil {
		return nil, err
	}

	bl, err := blk.BalanceOf(a.address)
	return types.NewBalance(bl), err
}

// Code returns the code of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already translates into computation
func (a *Account) Code() types.Code {
	code, err := a.code()
	panicOnError(err)
	return code
}

func (a *Account) code() (types.Code, error) {
	blk, err := a.fch.emulator.NewReadOnlyBlockView()
	if err != nil {
		return nil, err
	}
	return blk.CodeOf(a.address)
}

// CodeHash returns the code hash of this account
//
// Note: we don't meter any extra computation given reading data
// from the storage already translates into computation
func (a *Account) CodeHash() []byte {
	codeHash, err := a.codeHash()
	panicOnError(err)
	return codeHash
}

func (a *Account) codeHash() ([]byte, error) {
	blk, err := a.fch.emulator.NewReadOnlyBlockView()
	if err != nil {
		return nil, err
	}
	return blk.CodeHashOf(a.address)
}

// Deposit deposits the token from the given vault into the flow evm main vault
// and update the account balance with the new amount
func (a *Account) Deposit(v *types.FLOWTokenVault) {
	defer a.fch.backend.StartChildSpan(trace.FVMEVMDeposit).End()

	bridge := a.fch.addressAllocator.NativeTokenBridgeAddress()
	bridgeAccount := a.fch.AccountByAddress(bridge, false)
	// Note: its not an authorized call
	res, err := a.fch.executeAndHandleCall(
		types.NewDepositCall(
			bridge,
			a.address,
			v.Balance(),
			bridgeAccount.Nonce(),
		),
		v.Balance(),
		false,
	)
	panicOnErrorOrInvalidOrFailedState(res, err)
}

// Withdraw deducts the balance from the account and
// withdraw and return flow token from the Flex main vault.
func (a *Account) Withdraw(b types.Balance) *types.FLOWTokenVault {
	defer a.fch.backend.StartChildSpan(trace.FVMEVMWithdraw).End()

	res, err := a.executeAndHandleAuthorizedCall(
		types.NewWithdrawCall(
			a.fch.addressAllocator.NativeTokenBridgeAddress(),
			a.address,
			b,
			a.Nonce(),
		),
		b,
		true,
	)
	panicOnErrorOrInvalidOrFailedState(res, err)

	return types.NewFlowTokenVault(b)
}

// Transfer transfers tokens between accounts
func (a *Account) Transfer(to types.Address, balance types.Balance) {
	res, err := a.executeAndHandleAuthorizedCall(
		types.NewTransferCall(
			a.address,
			to,
			balance,
			a.Nonce(),
		),
		nil,
		false,
	)
	panicOnErrorOrInvalidOrFailedState(res, err)
}

// Deploy deploys a contract to the EVM environment
// the new deployed contract would be at the returned address
// contained in the result summary as data and
// the contract data is not controlled by the caller accounts
func (a *Account) Deploy(code types.Code, gaslimit types.GasLimit, balance types.Balance) *types.ResultSummary {
	// capture open tracing span
	defer a.fch.backend.StartChildSpan(trace.FVMEVMDeploy).End()

	res, err := a.executeAndHandleAuthorizedCall(
		types.NewDeployCall(
			a.address,
			code,
			uint64(gaslimit),
			balance,
			a.Nonce(),
		),
		nil,
		false,
	)
	panicOnError(err)

	return res.ResultSummary()
}

// Call calls a smart contract function with the given data
// it would limit the gas used according to the limit provided
// given it doesn't goes beyond what Flow transaction allows.
// the balance would be deducted from the OFA account and would be transferred to the target address
func (a *Account) Call(to types.Address, data types.Data, gaslimit types.GasLimit, balance types.Balance) *types.ResultSummary {
	// capture open tracing span
	defer a.fch.backend.StartChildSpan(trace.FVMEVMCall).End()

	res, err := a.executeAndHandleAuthorizedCall(
		types.NewContractCall(
			a.address,
			to,
			data,
			uint64(gaslimit),
			balance,
			a.Nonce(),
		),
		nil,
		false,
	)
	panicOnError(err)

	return res.ResultSummary()
}

func (a *Account) executeAndHandleAuthorizedCall(
	call *types.DirectCall,
	totalSupplyDiff *big.Int,
	deductSupplyDiff bool,
) (*types.Result, error) {
	if !a.isAuthorized {
		return nil, types.ErrUnauthorizedMethodCall
	}
	return a.fch.executeAndHandleCall(call, totalSupplyDiff, deductSupplyDiff)
}

func panicOnErrorOrInvalidOrFailedState(res *types.Result, err error) {

	if res != nil && res.Invalid() {
		panic(fvmErrors.NewEVMError(res.ValidationError))
	}

	if res != nil && res.Failed() {
		panic(fvmErrors.NewEVMError(res.VMError))
	}

	// this should never happen
	if err == nil && res == nil {
		panic(fvmErrors.NewEVMError(types.ErrUnexpectedEmptyResult))
	}

	panicOnError(err)
}

// panicOnError errors panic on returned errors
func panicOnError(err error) {
	if err == nil {
		return
	}

	if types.IsAFatalError(err) {
		panic(fvmErrors.NewEVMFailure(err))
	}

	if types.IsABackendError(err) {
		// backend errors doesn't need wrapping
		panic(err)
	}

	// any other returned errors are non-fatal errors
	panic(fvmErrors.NewEVMError(err))
}
