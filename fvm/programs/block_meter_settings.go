package programs

import (
	"fmt"

	"github.com/onflow/flow-go/fvm/meter"
	"github.com/onflow/flow-go/fvm/state"
)

const OCCKeyMeterParams = 0

type BlockMeterSettings struct {
	txnPrograms *TransactionPrograms

	cachedValue *OCCBlockItem[int, meter.MeterParameters]
	cachedState *state.State
}

func NewBlockMeterSettings() *BlockMeterSettings {
	occBlockItem, err := NewEmptyOCCBlock[int, meter.MeterParameters]().
		NewOCCBlockItem(0, 0)
	if err != nil {
		return nil
	}
	return &BlockMeterSettings{
		txnPrograms: nil,
		cachedValue: occBlockItem,
	}
}

func (block *BlockMeterSettings) NewChildBlockMeterSettings() *BlockMeterSettings {
	return &BlockMeterSettings{
		txnPrograms: block.txnPrograms,
		cachedValue: block.cachedValue,
		cachedState: block.cachedState,
	}
}

func (block *BlockMeterSettings) SetCurrentTransactionProgram(
	txnPrograms *TransactionPrograms,
) {
	block.txnPrograms = txnPrograms
}

func (block *BlockMeterSettings) GetOrRetrive(
	view state.View,
	retriever func(*state.TransactionState) (meter.MeterParameters, error),
	stateParams state.StateParameters,
) (*meter.MeterParameters, error) {
	if view == nil {
		return nil, fmt.Errorf("view parameter should not be nil")
	}
	if retriever == nil {
		return nil, fmt.Errorf("retriever closure should not be nil")
	}

	transactionState := state.NewTransactionState(view, stateParams)

	if block.cachedState == nil {
		nestedTxId, err := transactionState.BeginNestedTransaction()
		if err != nil {
			return block.cachedValue.Get(OCCKeyMeterParams), fmt.Errorf("failed to start a nested tx: %w", err)
		}

		// retrieve state value by calling retriever closure
		meterParams, err := retriever(transactionState)
		if err != nil {
			return block.cachedValue.Get(OCCKeyMeterParams), fmt.Errorf("failed to retrieve value: %w", err)
		}

		// apply reg touches on the target view and cache the delta state for replay purposes
		block.cachedState, err = transactionState.Commit(nestedTxId)
		if err != nil {
			block.cachedState = nil
			return block.cachedValue.Get(OCCKeyMeterParams), fmt.Errorf("failed to commit state: %w", err)
		}

		// commit retrieved value to
		block.cachedValue.Set(OCCKeyMeterParams, meterParams)
		if err != nil {
			block.cachedState = nil
			return block.cachedValue.Get(OCCKeyMeterParams), fmt.Errorf("failed to commit value: %w", err)
		}
	} else {
		// if value/state is cached already, simply replay it on the target view.
		err := transactionState.AttachAndCommit(block.cachedState)
		if err != nil {
			return block.cachedValue.Get(OCCKeyMeterParams), fmt.Errorf("failed to replay cached state: %w", err)
		}
	}

	return block.cachedValue.Get(OCCKeyMeterParams), nil
}

func (block *BlockMeterSettings) Commit() error {
	return block.cachedValue.Commit()
}

func (block *BlockMeterSettings) Invalidate() {
	block.cachedState = nil
	block.cachedValue.AddInvalidator(OCCMeterSettingsInvalidator{
		MeterSettingMutated: true,
	})
	if block.txnPrograms != nil {
		block.txnPrograms.AddInvalidator(OCCProgramsInvalidator{
			// TODO:
			ContractUpdateKeys: []ContractUpdateKey{
				{
					Name: "dummy",
				},
			},
		})
	}
}
