package query

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
	gethTracers "github.com/onflow/go-ethereum/eth/tracers"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

// View provides query capabilities over
// an specific state of the EVM chain.
type View struct {
	chainID         flow.ChainID
	rootAddr        flow.Address
	storage         *storage.EphemeralStorage
	blockSnapshot   types.BlockSnapshot
	tracer          *gethTracers.Tracer
	extraPCs        []types.PrecompiledContract
	maxCallGasLimit uint64
}

// NewView constructs a new view.
func NewView(
	chainID flow.ChainID,
	rootAddr flow.Address,
	storage *storage.EphemeralStorage,
	blockSnapshot types.BlockSnapshot,
	maxCallGasLimit uint64,
) *View {
	return &View{
		chainID:         chainID,
		rootAddr:        rootAddr,
		storage:         storage,
		blockSnapshot:   blockSnapshot,
		maxCallGasLimit: maxCallGasLimit,
	}
}

// GetBalance returns the balance for the given address
// can be used for the `eth_getBalance` endpoint
func (v *View) GetBalance(addr gethCommon.Address) (*big.Int, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return nil, err
	}
	bal, err := bv.GetBalance(addr)
	if err != nil {
		return nil, err
	}
	return bal.ToBig(), nil
}

// GetNonce returns the nonce for the given address
// can be used for the `eth_getTransactionCount` endpoint
func (v *View) GetNonce(addr gethCommon.Address) (uint64, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return 0, err
	}
	return bv.GetNonce(addr)
}

// GetCode returns the code for the given address
// can be used for the `eth_getCode` endpoint
func (v *View) GetCode(addr gethCommon.Address) ([]byte, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return nil, err
	}
	return bv.GetCode(addr)
}

// GetCodeHash returns the codehash for the given address
func (v *View) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	return bv.GetCodeHash(addr)
}

// GetSlab returns the slab for the given address and key
// can be used for the `eth_getStorageAt` endpoint
func (v *View) GetSlab(addr gethCommon.Address, key gethCommon.Hash) (gethCommon.Hash, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	return bv.GetState(types.SlotAddress{
		Address: addr,
		Key:     key,
	})
}

// DryCall runs a call off-chain and returns the results
// accepts override storage and precompiled call options
// as well as custom tracer.
func (v *View) DryCall(
	from gethCommon.Address,
	to gethCommon.Address,
	data []byte,
	value *big.Int,
	gasLimit uint64,
	opts ...DryCallOption,
) (*types.Result, error) {

	if gasLimit > v.maxCallGasLimit {
		return nil, fmt.Errorf(
			"gas limit is bigger than max gas limit allowed %d > %d",
			gasLimit, v.maxCallGasLimit,
		)
	}

	// apply all the options
	for _, op := range opts {
		err := op(v)
		if err != nil {
			return nil, err
		}
	}

	// create context
	ctx, err := v.blockSnapshot.BlockContext()
	if err != nil {
		return nil, err
	}
	ctx.Tracer = v.tracer
	ctx.ExtraPrecompiledContracts = v.extraPCs

	// create emulator
	em := emulator.NewEmulator(v.storage, v.rootAddr)

	// create a new block view
	bv, err := em.NewBlockView(ctx)
	if err != nil {
		return nil, err
	}

	res, err := bv.DirectCall(
		&types.DirectCall{
			From:     types.NewAddress(from),
			To:       types.NewAddress(to),
			Data:     data,
			Value:    value,
			GasLimit: gasLimit,
		},
	)
	if err != nil {
		return nil, err
	}

	return res, nil
}

// DryRunOption captures a options
// to be applied before the execution of a dry call.
type DryCallOption func(v *View) error

// WithStateOverrideBalance constructs a dry call option
// that replaces the balance of an address before the execution a dry call.
func WithStateOverrideBalance(
	addr gethCommon.Address,
	balance *big.Int,
) DryCallOption {
	return func(v *View) error {
		baseView, err := state.NewBaseView(v.storage, v.rootAddr)
		if err != nil {
			return err
		}
		nonce, err := baseView.GetNonce(addr)
		if err != nil {
			return err
		}
		code, err := baseView.GetCode(addr)
		if err != nil {
			return err
		}
		codeHash, err := baseView.GetCodeHash(addr)
		if err != nil {
			return err
		}

		convertedBalance, overflow := uint256.FromBig(balance)
		if overflow {
			return errors.New("balance too large")
		}

		err = baseView.UpdateAccount(addr, convertedBalance, nonce, code, codeHash)
		if err != nil {
			return err
		}
		return baseView.Commit()
	}
}

// WithStateOverrideNonce constructs a dry call option
// that replaces the nonce of an address before the execution a dry call.
func WithStateOverrideNonce(
	addr gethCommon.Address,
	nonce uint64,
) DryCallOption {
	return func(v *View) error {
		baseView, err := state.NewBaseView(v.storage, v.rootAddr)
		if err != nil {
			return err
		}
		balance, err := baseView.GetBalance(addr)
		if err != nil {
			return err
		}
		code, err := baseView.GetCode(addr)
		if err != nil {
			return err
		}
		codeHash, err := baseView.GetCodeHash(addr)
		if err != nil {
			return err
		}
		err = baseView.UpdateAccount(addr, balance, nonce, code, codeHash)
		if err != nil {
			return err
		}
		return baseView.Commit()
	}
}

// WithStateOverrideCode constructs a dry call option
// that replaces the code of an address before the dry call.
func WithStateOverrideCode(
	addr gethCommon.Address,
	code []byte,
) DryCallOption {
	return func(v *View) error {
		baseView, err := state.NewBaseView(v.storage, v.rootAddr)
		if err != nil {
			return err
		}
		balance, err := baseView.GetBalance(addr)
		if err != nil {
			return err
		}
		nonce, err := baseView.GetNonce(addr)
		if err != nil {
			return err
		}
		codeHash := gethTypes.EmptyCodeHash
		if len(code) > 0 {
			codeHash = gethCrypto.Keccak256Hash(code)
		}
		err = baseView.UpdateAccount(addr, balance, nonce, code, codeHash)
		if err != nil {
			return err
		}
		return baseView.Commit()
	}
}

// WithStateOverrideState constructs a dry call option
// that overrides all slots in the account storage before executing the call.
func WithStateOverrideState(
	addr gethCommon.Address,
	slots map[gethCommon.Hash]gethCommon.Hash,
) DryCallOption {
	return func(v *View) error {
		baseView, err := state.NewBaseView(v.storage, v.rootAddr)
		if err != nil {
			return err
		}
		// purge all the slots
		err = baseView.PurgeAllSlotsOfAnAccount(addr)
		if err != nil {
			return err
		}
		// no need to be sorted this is off-chain operation
		for k, v := range slots {
			err = baseView.UpdateSlot(types.SlotAddress{
				Address: addr,
				Key:     k,
			}, v)
			if err != nil {
				return err
			}
		}
		return baseView.Commit()
	}
}

// WithStateOverrideStateDiff constructs a dry call option
// that overrides slots of an account before executing the call.
func WithStateOverrideStateDiff(
	addr gethCommon.Address,
	slots map[gethCommon.Hash]gethCommon.Hash,
) DryCallOption {
	return func(v *View) error {
		baseView, err := state.NewBaseView(v.storage, v.rootAddr)
		if err != nil {
			return err
		}
		// no need to be sorted this is off-chain operation
		for k, v := range slots {
			err = baseView.UpdateSlot(types.SlotAddress{
				Address: addr,
				Key:     k,
			}, v)
			if err != nil {
				return err
			}
		}
		return baseView.Commit()
	}
}

// WithTracer constructs a dry call option
// that allows running the dry call with the
// custom tracer.
func WithTracer(
	tracer *gethTracers.Tracer,
) DryCallOption {
	return func(v *View) error {
		v.tracer = tracer
		return nil
	}
}

// WithExtraPrecompiledContracts constructs a dry call option
// that allows adding the precompiled contracts
// while executing the dry-call.
//
// this method can be used with remote PC caller for cadence arch calls
func WithExtraPrecompiledContracts(pcs []types.PrecompiledContract) DryCallOption {
	return func(v *View) error {
		v.extraPCs = pcs
		return nil
	}
}
