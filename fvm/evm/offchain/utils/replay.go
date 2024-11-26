package utils

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	evmStorage "github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/model/flow"
)

func ReplayEVMEventsToStore(
	log zerolog.Logger,
	store environment.ValueStore,
	chainID flow.ChainID,
	rootAddr flow.Address,
	evmBlockEvent *events.BlockEventPayload, // EVM block event
	evmTxEvents []events.TransactionEventPayload, // EVM transaction event
) (
	map[flow.RegisterID]flow.RegisterValue, // EVM state transition updates
	map[flow.RegisterID]flow.RegisterValue, // block provider updates
	error,
) {

	bpStorage := evmStorage.NewEphemeralStorage(store)
	bp, err := blocks.NewBasicProvider(chainID, bpStorage, rootAddr)
	if err != nil {
		return nil, nil, err
	}

	err = bp.OnBlockReceived(evmBlockEvent)
	if err != nil {
		return nil, nil, err
	}

	sp := testutils.NewTestStorageProvider(store, evmBlockEvent.Height)
	cr := sync.NewReplayer(chainID, rootAddr, sp, bp, log, nil, true)
	res, results, err := cr.ReplayBlock(evmTxEvents, evmBlockEvent)
	if err != nil {
		return nil, nil, err
	}

	// commit all register changes from the EVM state transition
	for k, v := range res.StorageRegisterUpdates() {
		err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
		if err != nil {
			return nil, nil, err
		}
	}

	blockProposal := blocks.ReconstructProposal(evmBlockEvent, results)

	err = bp.OnBlockExecuted(evmBlockEvent.Height, res, blockProposal)
	if err != nil {
		return nil, nil, err
	}

	// commit all register changes from non-EVM state transition, such
	// as block hash list changes
	for k, v := range bpStorage.StorageRegisterUpdates() {
		// verify the block hash list changes are included in the trie update

		err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
		if err != nil {
			return nil, nil, err
		}
	}

	return res.StorageRegisterUpdates(), bpStorage.StorageRegisterUpdates(), nil
}

type EVMEventsAccumulator struct {
	pendingEVMTxEvents []events.TransactionEventPayload
}

func NewEVMEventsAccumulator() *EVMEventsAccumulator {
	return &EVMEventsAccumulator{
		pendingEVMTxEvents: make([]events.TransactionEventPayload, 0),
	}
}

func (a *EVMEventsAccumulator) HasBlockEvent(
	evmBlockEvent *events.BlockEventPayload,
	evmTxEvents []events.TransactionEventPayload) (
	*events.BlockEventPayload,
	[]events.TransactionEventPayload,
	bool, // true if there is an EVM block event
) {
	a.pendingEVMTxEvents = append(a.pendingEVMTxEvents, evmTxEvents...)

	// if there is no EVM block event, we will accumulate the pending txs
	if evmBlockEvent == nil {
		return evmBlockEvent, a.pendingEVMTxEvents, false
	}

	// if there is an EVM block event, we return the EVM block and the accumulated tx events
	return evmBlockEvent, a.pendingEVMTxEvents, true
}
