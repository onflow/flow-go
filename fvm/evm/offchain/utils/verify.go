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
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/storage"
)

// EVM Root Height is the first block that has EVM Block Event where the EVM block height is 1
func IsEVMRootHeight(chainID flow.ChainID, flowHeight uint64) bool {
	if chainID == flow.Testnet {
		return flowHeight == 211176670
	} else if chainID == flow.Mainnet {
		return flowHeight == 85981135
	}
	return flowHeight == 1
}

// IsSporkHeight returns true if the given flow height is a spork height for the given chainID
// At spork height, there is no EVM events
func IsSporkHeight(chainID flow.ChainID, flowHeight uint64) bool {
	if IsEVMRootHeight(chainID, flowHeight) {
		return true
	}

	if chainID == flow.Testnet {
		return flowHeight == 218215349 // Testnet 52
	} else if chainID == flow.Mainnet {
		return flowHeight == 88226267 // Mainnet 26
	}
	return false
}

// OffchainReplayBackwardCompatibilityTest replays the offchain EVM state transition for a given range of flow blocks,
// the replay will also verify the StateUpdateChecksum of the EVM state transition from each transaction execution.
// the updated register values will be saved to the given value store.
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

	// pendingEVMTxEvents are tx events that are executed block included in a flow block that
	// didn't emit EVM block event, which is caused when the system tx to emit EVM block fails.
	// we accumulate these pending txs, and replay them when we encounter a block with EVM block event.
	pendingEVMEvents := NewEVMEventsAccumulator()

	for height := flowStartHeight; height <= flowEndHeight; height++ {
		// account status initialization for the root account at the EVM root height
		if IsEVMRootHeight(chainID, height) {
			log.Info().Msgf("initializing EVM state for root height %d", flowStartHeight)

			as := environment.NewAccountStatus()
			rootAddr := evm.StorageAccountAddress(chainID)
			err := store.SetValue(rootAddr[:], []byte(flow.AccountStatusKey), as.ToBytes())
			if err != nil {
				return err
			}

			continue
		}

		if IsSporkHeight(chainID, height) {
			// spork root block has no EVM events
			continue
		}

		// get EVM events and register updates at the flow height
		evmBlockEvent, evmTxEvents, registerUpdates, err := evmEventsAndRegisterUpdatesAtFlowHeight(
			height,
			headers, results, executionDataStore, rootAddrStr)
		if err != nil {
			return fmt.Errorf("failed to get EVM events and register updates at height %d: %w", height, err)
		}

		blockEvent, txEvents, hasBlockEvent := pendingEVMEvents.HasBlockEvent(evmBlockEvent, evmTxEvents)

		if !hasBlockEvent {
			log.Info().Msgf("block has no EVM block, height :%v, txEvents: %v", height, len(evmTxEvents))

			err = onHeightReplayed(height)
			if err != nil {
				return err
			}
			continue
		}

		evmUpdates, blockProviderUpdates, err := ReplayEVMEventsToStore(
			log,
			store,
			chainID,
			rootAddr,
			blockEvent,
			txEvents,
		)
		if err != nil {
			return fmt.Errorf("fail to replay events: %w", err)
		}

		err = verifyEVMRegisterUpdates(registerUpdates, evmUpdates, blockProviderUpdates)
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

func evmEventsAndRegisterUpdatesAtFlowHeight(
	flowHeight uint64,
	headers storage.Headers,
	results storage.ExecutionResults,
	executionDataStore execution_data.ExecutionDataGetter,
	rootAddr string,
) (
	*events.BlockEventPayload, // EVM block event, might be nil if there is no block Event at this height
	[]events.TransactionEventPayload, // EVM transaction event
	map[flow.RegisterID]flow.RegisterValue, // update registers
	error,
) {

	blockID, err := headers.BlockIDByHeight(flowHeight)
	if err != nil {
		return nil, nil, nil, err
	}

	result, err := results.ByBlockID(blockID)
	if err != nil {
		return nil, nil, nil, err
	}

	executionData, err := executionDataStore.Get(context.Background(), result.ExecutionDataID)
	if err != nil {
		return nil, nil, nil,
			fmt.Errorf("could not get execution data %v for block %d: %w",
				result.ExecutionDataID, flowHeight, err)
	}

	evts := flow.EventsList{}
	payloads := []*ledger.Payload{}

	for _, chunkData := range executionData.ChunkExecutionDatas {
		evts = append(evts, chunkData.Events...)
		payloads = append(payloads, chunkData.TrieUpdate.Payloads...)
	}

	updates := make(map[flow.RegisterID]flow.RegisterValue, len(payloads))
	for i := len(payloads) - 1; i >= 0; i-- {
		regID, regVal, err := convert.PayloadToRegister(payloads[i])
		if err != nil {
			return nil, nil, nil, err
		}

		// find the register updates for the root account
		if regID.Owner == rootAddr {
			updates[regID] = regVal
		}
	}

	// parse EVM events
	evmBlockEvent, evmTxEvents, err := parseEVMEvents(evts)
	if err != nil {
		return nil, nil, nil, err
	}
	return evmBlockEvent, evmTxEvents, updates, nil
}

func verifyEVMRegisterUpdates(
	registerUpdates map[flow.RegisterID]flow.RegisterValue,
	evmUpdates map[flow.RegisterID]flow.RegisterValue,
	blockProviderUpdates map[flow.RegisterID]flow.RegisterValue,
) error {
	// skip the register level validation
	// since the register is not stored at the same slab id as the on-chain EVM
	// instead, we will compare by exporting the logic EVM state, which contains
	// accounts, codes and slots.
	return nil
}

func VerifyRegisterUpdates(expectedUpdates map[flow.RegisterID]flow.RegisterValue, actualUpdates map[flow.RegisterID]flow.RegisterValue) error {
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
