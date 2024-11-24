package utils

import (
	"context"
	"errors"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/evm"
	"github.com/onflow/flow-go/fvm/evm/events"
	"github.com/onflow/flow-go/fvm/evm/offchain/blocks"
	evmStorage "github.com/onflow/flow-go/fvm/evm/offchain/storage"
	"github.com/onflow/flow-go/fvm/evm/offchain/sync"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

func OffchainReplayBackwardCompatibilityTest(
	chainID flow.ChainID,
	flowStartHeight uint64,
	flowEndHeight uint64,
	headers storage.Headers,
	results storage.ExecutionResults,
	executionDataStore execution_data.ExecutionDataGetter,
	store environment.ValueStore,
) error {
	rootAddr := evm.StorageAccountAddress(chainID)
	rootAddrStr := string(rootAddr.Bytes())

	bpStorage := evmStorage.NewEphemeralStorage(store)
	bp, err := blocks.NewBasicProvider(chainID, bpStorage, rootAddr)
	if err != nil {
		return err
	}

	for height := flowStartHeight; height <= flowEndHeight; height++ {
		blockID, err := headers.BlockIDByHeight(height)
		if err != nil {
			return err
		}

		result, err := results.ByBlockID(blockID)
		if err != nil {
			return err
		}

		executionData, err := executionDataStore.Get(context.Background(), result.ExecutionDataID)
		if err != nil {
			return err
		}

		events := flow.EventsList{}
		payloads := []*ledger.Payload{}

		for _, chunkData := range executionData.ChunkExecutionDatas {
			events = append(events, chunkData.Events...)
			payloads = append(payloads, chunkData.TrieUpdate.Payloads...)
		}

		updates := make(map[flow.RegisterID]flow.RegisterValue, len(payloads))
		for i := len(payloads) - 1; i >= 0; i-- {
			regID, regVal, err := convert.PayloadToRegister(payloads[i])
			if err != nil {
				return err
			}

			// skip non-evm-account registers
			if regID.Owner != rootAddrStr {
				continue
			}

			// when iterating backwards, duplicated register updates are stale updates,
			// so skipping them
			if _, ok := updates[regID]; !ok {
				updates[regID] = regVal
			}
		}

		// parse events
		evmBlockEvent, evmTxEvents, err := parseEVMEvents(events)
		if err != nil {
			return err
		}

		err = bp.OnBlockReceived(evmBlockEvent)
		if err != nil {
			return err
		}

		sp := testutils.NewTestStorageProvider(store, evmBlockEvent.Height)
		cr := sync.NewReplayer(chainID, rootAddr, sp, bp, log.Logger, nil, true)
		res, err := cr.ReplayBlock(evmTxEvents, evmBlockEvent)
		if err != nil {
			return err
		}

		// commit all changes
		for k, v := range res.StorageRegisterUpdates() {
			err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
			if err != nil {
				return err
			}
		}

		err = bp.OnBlockExecuted(evmBlockEvent.Height, res)
		if err != nil {
			return err
		}

		// verify and commit all block hash list changes
		for k, v := range bpStorage.StorageRegisterUpdates() {
			// verify the block hash list changes are included in the trie update

			err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
			if err != nil {
				return err
			}
		}

		log.Info().Msgf("verified block %d (EVM height)", height, evmBlockEvent.Height)
	}

	return nil
}

func parseEVMEvents(evts flow.EventsList) (*events.BlockEventPayload, []events.TransactionEventPayload, error) {
	var blockEvent *events.BlockEventPayload
	txEvents := make([]events.TransactionEventPayload, 0)

	for _, e := range evts {
		evtType := string(e.Type)
		if strings.Contains(evtType, "BlockExecuted") {
			if blockEvent != nil {
				return nil, nil, errors.New("multiple block events in a single block")
			}

			ev, err := ccf.Decode(nil, e.Payload)
			if err != nil {
				return nil, nil, err
			}

			blockEventPayload, err := events.DecodeBlockEventPayload(ev.(cadence.Event))
			if err != nil {
				return nil, nil, err
			}
			blockEvent = blockEventPayload
		} else if strings.Contains(evtType, "TransactionExecuted") {
			ev, err := ccf.Decode(nil, e.Payload)
			if err != nil {
				return nil, nil, err
			}
			txEv, err := events.DecodeTransactionEventPayload(ev.(cadence.Event))
			if err != nil {
				return nil, nil, err
			}
			txEvents = append(txEvents, *txEv)
		}
	}

	return blockEvent, txEvents, nil
}
