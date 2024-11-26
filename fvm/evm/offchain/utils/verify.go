package utils

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/rs/zerolog"

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

// EVM Root Height is the first block that has EVM Block Event where the EVM block height is 1
func IsEVMRootHeight(chainID flow.ChainID, flowHeight uint64) bool {
	if chainID == flow.Testnet {
		return flowHeight == 211176671
	} else if chainID == flow.Mainnet {
		return flowHeight == 85981136
	}
	return flowHeight == 1
}

func OffchainReplayBackwardCompatibilityTest(
	log zerolog.Logger,
	chainID flow.ChainID,
	flowStartHeight uint64,
	flowEndHeight uint64,
	headers storage.Headers,
	results storage.ExecutionResults,
	executionDataStore execution_data.ExecutionDataGetter,
	store environment.ValueStore,
	onHeightReplayed func(uint64) error,
) error {
	rootAddr := evm.StorageAccountAddress(chainID)
	rootAddrStr := string(rootAddr.Bytes())

	if IsEVMRootHeight(chainID, flowStartHeight) {
		log.Info().Msgf("initializing EVM state for root height %d", flowStartHeight)

		as := environment.NewAccountStatus()
		rootAddr := evm.StorageAccountAddress(chainID)
		err := store.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
		if err != nil {
			return err
		}
	}

	// pendingEVMTxEvents are tx events that are executed block included in a flow block that
	// didn't emit EVM block event, which is caused when the system tx to emit EVM block fails.
	// we accumulate these pending txs, and replay them when we encounter a block with EVM block event.
	pendingEVMTxEvents := make([]events.TransactionEventPayload, 0)

	for height := flowStartHeight; height <= flowEndHeight; height++ {
		bpStorage := evmStorage.NewEphemeralStorage(store)
		bp, err := blocks.NewBasicProvider(chainID, bpStorage, rootAddr)
		if err != nil {
			return err
		}

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
			return fmt.Errorf("could not get execution data %v for block %d: %w", result.ExecutionDataID, height, err)
		}

		evts := flow.EventsList{}
		payloads := []*ledger.Payload{}

		for _, chunkData := range executionData.ChunkExecutionDatas {
			evts = append(evts, chunkData.Events...)
			payloads = append(payloads, chunkData.TrieUpdate.Payloads...)
		}

		expectedUpdates := make(map[flow.RegisterID]flow.RegisterValue, len(payloads))
		for i := len(payloads) - 1; i >= 0; i-- {
			regID, regVal, err := convert.PayloadToRegister(payloads[i])
			if err != nil {
				return err
			}

			// skip non-evm-account registers
			if regID.Owner != rootAddrStr {
				continue
			}

			if !verifiableKeys(regID) {
				continue
			}

			// when iterating backwards, duplicated register updates are stale updates,
			// so skipping them
			if _, ok := expectedUpdates[regID]; !ok {
				expectedUpdates[regID] = regVal
			}
		}

		// parse EVM events
		evmBlockEvent, evmTxEvents, err := parseEVMEvents(evts)
		if err != nil {
			return err
		}

		pendingEVMTxEvents = append(pendingEVMTxEvents, evmTxEvents...)

		if evmBlockEvent == nil {
			log.Info().Msgf("block has no EVM block, height :%v, txEvents: %v", height, len(evmTxEvents))

			err = onHeightReplayed(height)
			if err != nil {
				return err
			}
			continue
		}

		// when we encounter a block with EVM block event, we replay the pending txs accumulated
		// from previous blocks that had no EVM block event.
		evmTxEventsIncludedInBlock := pendingEVMTxEvents
		// reset pendingEVMTxEvents
		pendingEVMTxEvents = make([]events.TransactionEventPayload, 0)

		err = bp.OnBlockReceived(evmBlockEvent)
		if err != nil {
			return err
		}

		sp := testutils.NewTestStorageProvider(store, evmBlockEvent.Height)
		cr := sync.NewReplayer(chainID, rootAddr, sp, bp, log, nil, true)
		res, results, err := cr.ReplayBlock(evmTxEventsIncludedInBlock, evmBlockEvent)
		if err != nil {
			return err
		}

		actualUpdates := make(map[flow.RegisterID]flow.RegisterValue, len(expectedUpdates))

		// commit all register changes from the EVM state transition
		for k, v := range res.StorageRegisterUpdates() {
			err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
			if err != nil {
				return err
			}

			if !verifiableKeys(k) {
				continue
			}

			actualUpdates[k] = v
		}

		blockProposal := blocks.ReconstructProposal(evmBlockEvent, results)

		err = bp.OnBlockExecuted(evmBlockEvent.Height, res, blockProposal)
		if err != nil {
			return err
		}

		// commit all register changes from non-EVM state transition, such
		// as block hash list changes
		for k, v := range bpStorage.StorageRegisterUpdates() {
			// verify the block hash list changes are included in the trie update

			err = store.SetValue([]byte(k.Owner), []byte(k.Key), v)
			if err != nil {
				return err
			}

			if !verifiableKeys(k) {
				continue
			}

			actualUpdates[k] = v
		}

		err = verifyRegisterUpdates(expectedUpdates, actualUpdates)
		if err != nil {
			return err
		}

		err = onHeightReplayed(height)
		if err != nil {
			return err
		}
	}

	return nil
}

func verifiableKeys(key flow.RegisterID) bool {
	return false
	// return handler.IsBlockHashListBucketKeyFormat(key) || handler.IsBlockHashListMetaKey(key)
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

func verifyRegisterUpdates(expectedUpdates map[flow.RegisterID]flow.RegisterValue, actualUpdates map[flow.RegisterID]flow.RegisterValue) error {
	missingUpdates := make(map[flow.RegisterID]flow.RegisterValue)
	additionalUpdates := make(map[flow.RegisterID]flow.RegisterValue)
	mismatchingUpdates := make(map[flow.RegisterID][2]flow.RegisterValue)

	for k, v := range expectedUpdates {
		if actualVal, ok := actualUpdates[k]; !ok {
			missingUpdates[k] = v
		} else if !bytes.Equal(v, actualVal) {
			mismatchingUpdates[k] = [2]flow.RegisterValue{v, actualVal}
		}

		delete(actualUpdates, k)
	}

	for k, v := range actualUpdates {
		additionalUpdates[k] = v
	}

	// Build a combined error message
	var errorMessage strings.Builder

	if len(missingUpdates) > 0 {
		errorMessage.WriteString("Missing register updates:\n")
		for id, value := range missingUpdates {
			errorMessage.WriteString(fmt.Sprintf("  RegisterKey: %v, ExpectedValue: %x\n", id.Key, value))
		}
	}

	if len(additionalUpdates) > 0 {
		errorMessage.WriteString("Additional register updates:\n")
		for id, value := range additionalUpdates {
			errorMessage.WriteString(fmt.Sprintf("  RegisterKey: %v, ActualValue: %x\n", id.Key, value))
		}
	}

	if len(mismatchingUpdates) > 0 {
		errorMessage.WriteString("Mismatching register updates:\n")
		for id, values := range mismatchingUpdates {
			errorMessage.WriteString(fmt.Sprintf("  RegisterKey: %v, ExpectedValue: %x, ActualValue: %x\n", id.Key, values[0], values[1]))
		}
	}

	if errorMessage.Len() > 0 {
		return errors.New(errorMessage.String())
	}

	return nil
}
