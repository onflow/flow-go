package query

import (
	"math/big"

	"github.com/holiman/uint256"
	gethCommon "github.com/onflow/go-ethereum/common"
	gethTypes "github.com/onflow/go-ethereum/core/types"
	gethCrypto "github.com/onflow/go-ethereum/crypto"
	gethTracers "github.com/onflow/go-ethereum/eth/tracers"

	"github.com/onflow/flow-go/fvm/evm/emulator"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	"github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

type View struct {
	chainID  flow.ChainID
	rootAddr flow.Address
	storage  *storage.EphemeralStorage
	tracer   *gethTracers.Tracer
	extraPCs []types.PrecompiledContract
}

func NewView(
	chainID flow.ChainID,
	rootAddr flow.Address,
	storage *storage.EphemeralStorage,
) *View {
	return &View{
		chainID:  chainID,
		rootAddr: rootAddr,
		storage:  storage,
	}
}

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

func (v *View) GetNonce(addr gethCommon.Address) (uint64, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return 0, err
	}
	return bv.GetNonce(addr)
}

func (v *View) GetCode(addr gethCommon.Address) ([]byte, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return nil, err
	}
	return bv.GetCode(addr)
}

func (v *View) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	bv, err := state.NewBaseView(v.storage, v.rootAddr)
	if err != nil {
		return gethCommon.Hash{}, err
	}
	return bv.GetCodeHash(addr)
}

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

func (v *View) DryCall(
	from gethCommon.Address,
	to gethCommon.Address,
	data []byte,
	value *big.Int,
	gasLimit uint64,
	gasPrice *big.Int,
	opts ...DryRunOption,
) (*types.Result, error) {
	// apply all the options
	for _, op := range opts {
		err := op(v)
		if err != nil {
			return nil, err
		}
	}

	blks, err := blocks.NewBlocks(v.chainID, v.rootAddr, v.storage)
	if err != nil {
		return nil, err
	}

	// create context
	ctx, err := sync.CreateBlockContext(v.chainID, blks, v.tracer)
	if err != nil {
		return nil, err
	}
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

type DryRunOption func(v *View) error

func NewDryRunStorageOverrideBalance(
	addr gethCommon.Address,
	balance *uint256.Int,
) DryRunOption {
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
		err = baseView.UpdateAccount(addr, balance, nonce, code, codeHash)
		if err != nil {
			return err
		}
		return baseView.Commit()
	}
}

func NewDryRunStorageOverrideNonce(
	addr gethCommon.Address,
	nonce uint64,
) DryRunOption {
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

func NewDryRunStorageOverrideCode(
	addr gethCommon.Address,
	code []byte,
) DryRunOption {
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

func NewDryRunStorageOverrideState(
	addr gethCommon.Address,
	slots map[gethCommon.Hash]gethCommon.Hash,
) DryRunOption {
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

func NewDryRunStorageOverrideStateDiff(
	addr gethCommon.Address,
	slots map[gethCommon.Hash]gethCommon.Hash,
) DryRunOption {
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

func NewDryRunTracerUpdate(
	tracer *gethTracers.Tracer,
) DryRunOption {
	return func(v *View) error {
		v.tracer = tracer
		return nil
	}
}

// this method can be used with remote PC caller for cadence arch calls
func NewDryRunWithExtraPrecompiledContracts(pcs []types.PrecompiledContract) DryRunOption {
	return func(v *View) error {
		v.extraPCs = pcs
		return nil
	}

}
