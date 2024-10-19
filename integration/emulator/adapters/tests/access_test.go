/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tests

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go/integration/emulator"
	"github.com/onflow/flow-go/integration/emulator/adapters"
	"github.com/onflow/flow-go/integration/emulator/mocks"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	flowgo "github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func accessTest(f func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator)) func(t *testing.T) {
	return func(t *testing.T) {
		mockCtrl := gomock.NewController(t)
		defer mockCtrl.Finish()

		emu := mocks.NewMockEmulator(mockCtrl)
		logger := zerolog.Nop()
		back := adapters.NewAccessAdapter(&logger, emu)

		f(t, back, emu)
	}
}

func TestAccess(t *testing.T) {

	t.Run("Ping", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {
		emu.EXPECT().
			Ping().
			Times(1)

		err := adapter.Ping(context.Background())
		assert.NoError(t, err)
	}))

	t.Run("GetNetworkParameters", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {
		expected := access.NetworkParameters{
			ChainID: flowgo.MonotonicEmulator,
		}

		emu.EXPECT().
			GetNetworkParameters().
			Return(expected).
			Times(1)

		result := adapter.GetNetworkParameters(context.Background())
		assert.Equal(t, expected, result)

	}))

	t.Run("GetLatestBlockHeader", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		header := flowgo.Header{
			Height: 42,
		}
		expected := flowgo.Block{Header: &header}

		//success
		emu.EXPECT().
			GetLatestBlock().
			Return(&expected, nil).
			Times(1)

		result, blockStatus, err := adapter.GetLatestBlockHeader(context.Background(), true)
		assert.Equal(t, expected.Header, result)
		assert.Equal(t, flowgo.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetLatestBlock().
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetLatestBlockHeader(context.Background(), true)
		assert.Nil(t, result)
		assert.Equal(t, flowgo.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockHeaderByHeight", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		header := flowgo.Header{
			Height: 42,
		}
		expected := flowgo.Block{Header: &header}

		//success
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(&expected, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockHeaderByHeight(context.Background(), 42)
		assert.Equal(t, expected.Header, result)
		assert.Equal(t, flowgo.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockHeaderByHeight(context.Background(), 42)
		assert.Nil(t, result)
		assert.Equal(t, flowgo.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockHeaderByID", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		id := flowgo.Identifier{}
		header := flowgo.Header{
			Height: 42,
		}
		expected := flowgo.Block{Header: &header}

		//success
		emu.EXPECT().
			GetBlockByID(id).
			Return(&expected, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockHeaderByID(context.Background(), id)
		assert.Equal(t, expected.Header, result)
		assert.Equal(t, flowgo.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByID(id).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockHeaderByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Equal(t, flowgo.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetLatestBlock", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		header := flowgo.Header{
			Height: 42,
		}
		expected := flowgo.Block{Header: &header}

		//success
		emu.EXPECT().
			GetLatestBlock().
			Return(&expected, nil).
			Times(1)

		result, blockStatus, err := adapter.GetLatestBlock(context.Background(), true)
		assert.Equal(t, expected, *result)
		assert.Equal(t, flowgo.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetLatestBlock().
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetLatestBlock(context.Background(), true)
		assert.Nil(t, result)
		assert.Equal(t, flowgo.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockByHeight", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		header := flowgo.Header{
			Height: 42,
		}
		expected := flowgo.Block{Header: &header}

		//success
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(&expected, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockByHeight(context.Background(), 42)
		assert.Equal(t, expected, *result)
		assert.Equal(t, flowgo.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByHeight(uint64(42)).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockByHeight(context.Background(), 42)
		assert.Nil(t, result)
		assert.Equal(t, flowgo.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetBlockByID", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		id := flowgo.Identifier{}
		header := flowgo.Header{
			Height: 42,
		}
		expected := flowgo.Block{Header: &header}

		//success
		emu.EXPECT().
			GetBlockByID(id).
			Return(&expected, nil).
			Times(1)

		result, blockStatus, err := adapter.GetBlockByID(context.Background(), id)
		assert.Equal(t, expected, *result)
		assert.Equal(t, flowgo.BlockStatusSealed, blockStatus)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetBlockByID(id).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, blockStatus, err = adapter.GetBlockByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Equal(t, flowgo.BlockStatusUnknown, blockStatus)
		assert.Error(t, err)

	}))

	t.Run("GetCollectionByID", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		id := flowgo.Identifier{}
		expected := flowgo.LightCollection{}

		//success
		emu.EXPECT().
			GetCollectionByID(id).
			Return(&expected, nil).
			Times(1)

		result, err := adapter.GetCollectionByID(context.Background(), id)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetCollectionByID(id).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetCollectionByID(context.Background(), id)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransaction", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		id := flowgo.Identifier{}
		expected := flowgo.TransactionBody{}

		//success
		emu.EXPECT().
			GetTransaction(id).
			Return(&expected, nil).
			Times(1)

		result, err := adapter.GetTransaction(context.Background(), id)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransaction(id).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransaction(context.Background(), id)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransactionResult", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		txID := flowgo.Identifier{}
		blockID := flowgo.Identifier{}
		collectionID := flowgo.Identifier{}

		emuResult := access.TransactionResult{
			Events: []flowgo.Event{
				ccfEventFixture(t),
			},
		}
		expected := access.TransactionResult{
			Events: []flowgo.Event{
				jsonCDCEventFixture(t),
			},
		}

		//success
		emu.EXPECT().
			GetTransactionResult(txID).
			Return(&emuResult, nil).
			Times(1)

		result, err := adapter.GetTransactionResult(context.Background(), txID, blockID, collectionID, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransactionResult(txID).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransactionResult(context.Background(), txID, blockID, collectionID, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetAccount", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		address := flowgo.Address{}
		expected := flowgo.Account{}

		//success
		emu.EXPECT().
			GetAccount(address).
			Return(&expected, nil).
			Times(1)

		result, err := adapter.GetAccount(context.Background(), address)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetAccount(address).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetAccount(context.Background(), address)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetAccountAtLatestBlock", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		address := flowgo.Address{}
		expected := flowgo.Account{}

		//success
		emu.EXPECT().
			GetAccount(address).
			Return(&expected, nil).
			Times(1)

		result, err := adapter.GetAccountAtLatestBlock(context.Background(), address)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetAccount(address).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetAccountAtLatestBlock(context.Background(), address)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetAccountAtBlockHeight", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		address := flowgo.Address{}
		height := uint64(42)

		expected := flowgo.Account{}

		//success
		emu.EXPECT().
			GetAccountAtBlockHeight(address, height).
			Return(&expected, nil).
			Times(1)

		result, err := adapter.GetAccountAtBlockHeight(context.Background(), address, height)
		assert.Equal(t, expected, *result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetAccountAtBlockHeight(address, height).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetAccountAtBlockHeight(context.Background(), address, height)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("ExecuteScriptAtLatestBlock", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		script := []byte("some cadence code here")
		var arguments [][]byte

		stringValue, _ := cadence.NewString("42")
		emulatorResult := emulator.ScriptResult{Value: stringValue}
		expected, _ := adapter.ConvertScriptResult(&emulatorResult, nil)

		// called once for each script execution
		emu.EXPECT().
			GetLatestBlock().
			Return(&flowgo.Block{Header: &flowgo.Header{}}, nil).
			Times(2)

		//success
		emu.EXPECT().
			ExecuteScript(script, arguments).
			Return(&emulatorResult, nil).
			Times(1)

		result, err := adapter.ExecuteScriptAtLatestBlock(context.Background(), script, arguments)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			ExecuteScript(script, arguments).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.ExecuteScriptAtLatestBlock(context.Background(), script, arguments)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("ExecuteScriptAtBlockHeight", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		script := []byte("some cadence code here")
		var arguments [][]byte

		height := uint64(42)
		stringValue, _ := cadence.NewString("42")
		emulatorResult := emulator.ScriptResult{Value: stringValue}
		expected, _ := adapter.ConvertScriptResult(&emulatorResult, nil)

		//success
		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, height).
			Return(&emulatorResult, nil).
			Times(1)

		result, err := adapter.ExecuteScriptAtBlockHeight(context.Background(), height, script, arguments)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			ExecuteScriptAtBlockHeight(script, arguments, height).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.ExecuteScriptAtBlockHeight(context.Background(), height, script, arguments)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("ExecuteScriptAtBlockID", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		script := []byte("some cadence code here")
		var arguments [][]byte

		id := flowgo.Identifier{}
		stringValue, _ := cadence.NewString("42")
		emulatorResult := emulator.ScriptResult{Value: stringValue}
		expected, _ := adapter.ConvertScriptResult(&emulatorResult, nil)

		//success
		emu.EXPECT().
			ExecuteScriptAtBlockID(script, arguments, id).
			Return(&emulatorResult, nil).
			Times(1)

		result, err := adapter.ExecuteScriptAtBlockID(context.Background(), id, script, arguments)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			ExecuteScriptAtBlockID(script, arguments, id).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.ExecuteScriptAtBlockID(context.Background(), id, script, arguments)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetEventsForHeightRange", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		eventType := "testEvent"
		startHeight := uint64(0)
		endHeight := uint64(42)

		blockEvents := []flowgo.BlockEvents{
			{
				Events: []flowgo.Event{
					ccfEventFixture(t),
				},
			},
		}
		expected := []flowgo.BlockEvents{
			{
				Events: []flowgo.Event{
					jsonCDCEventFixture(t),
				},
			},
		}

		//success
		emu.EXPECT().
			GetEventsForHeightRange(eventType, startHeight, endHeight).
			Return(blockEvents, nil).
			Times(1)

		result, err := adapter.GetEventsForHeightRange(context.Background(), eventType, startHeight, endHeight, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetEventsForHeightRange(eventType, startHeight, endHeight).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetEventsForHeightRange(context.Background(), eventType, startHeight, endHeight, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetEventsForBlockIDs", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		eventType := "testEvent"
		blockIDs := []flowgo.Identifier{flowgo.Identifier{}}

		blockEvents := []flowgo.BlockEvents{
			{
				Events: []flowgo.Event{
					ccfEventFixture(t),
				},
			},
		}
		expected := []flowgo.BlockEvents{
			{
				Events: []flowgo.Event{
					jsonCDCEventFixture(t),
				},
			},
		}

		//success
		emu.EXPECT().
			GetEventsForBlockIDs(eventType, blockIDs).
			Return(blockEvents, nil).
			Times(1)

		result, err := adapter.GetEventsForBlockIDs(context.Background(), eventType, blockIDs, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetEventsForBlockIDs(eventType, blockIDs).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetEventsForBlockIDs(context.Background(), eventType, blockIDs, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransactionResultByIndex", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		blockID := flowgo.Identifier{}
		index := uint32(0)

		txResult := &access.TransactionResult{
			Events: []flowgo.Event{
				ccfEventFixture(t),
			},
		}
		results := []*access.TransactionResult{txResult}
		convertedTXResult := &access.TransactionResult{
			Events: []flowgo.Event{
				jsonCDCEventFixture(t),
			},
		}

		//success
		emu.EXPECT().
			GetTransactionResultsByBlockID(blockID).
			Return(results, nil).
			Times(1)

		result, err := adapter.GetTransactionResultByIndex(context.Background(), blockID, index, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Equal(t, convertedTXResult, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransactionResultsByBlockID(blockID).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransactionResultByIndex(context.Background(), blockID, index, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransactionsByBlockID", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		blockID := flowgo.Identifier{}

		expected := []*flowgo.TransactionBody{}

		//success
		emu.EXPECT().
			GetTransactionsByBlockID(blockID).
			Return(expected, nil).
			Times(1)

		result, err := adapter.GetTransactionsByBlockID(context.Background(), blockID)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransactionsByBlockID(blockID).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransactionsByBlockID(context.Background(), blockID)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("GetTransactionResultsByBlockID", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		blockID := flowgo.Identifier{}

		results := []*access.TransactionResult{
			{
				Events: []flowgo.Event{
					ccfEventFixture(t),
				},
			},
		}
		expected := []*access.TransactionResult{
			{
				Events: []flowgo.Event{
					jsonCDCEventFixture(t),
				},
			},
		}

		//success
		emu.EXPECT().
			GetTransactionResultsByBlockID(blockID).
			Return(results, nil).
			Times(1)

		result, err := adapter.GetTransactionResultsByBlockID(context.Background(), blockID, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Equal(t, expected, result)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			GetTransactionResultsByBlockID(blockID).
			Return(nil, fmt.Errorf("some error")).
			Times(1)

		result, err = adapter.GetTransactionResultsByBlockID(context.Background(), blockID, entities.EventEncodingVersion_JSON_CDC_V0)
		assert.Nil(t, result)
		assert.Error(t, err)

	}))

	t.Run("SendTransaction", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {

		transaction := flowgo.TransactionBody{}

		//success
		emu.EXPECT().
			SendTransaction(&transaction).
			Return(nil).
			Times(1)

		err := adapter.SendTransaction(context.Background(), &transaction)
		assert.NoError(t, err)

		//fail
		emu.EXPECT().
			SendTransaction(&transaction).
			Return(fmt.Errorf("some error")).
			Times(1)

		err = adapter.SendTransaction(context.Background(), &transaction)
		assert.Error(t, err)

	}))

	t.Run("GetNodeVersionInfo", accessTest(func(t *testing.T, adapter *adapters.AccessAdapter, emu *mocks.MockEmulator) {
		info, err := adapter.GetNodeVersionInfo(context.Background())
		assert.NoError(t, err)
		require.Equal(t, uint64(0), info.NodeRootBlockHeight)
	}))

}

func ccfEventFixture(t *testing.T) flowgo.Event {
	cadenceValue := cadence.NewInt(2)

	ccfEvent, err := ccf.EventsEncMode.Encode(cadenceValue)
	require.NoError(t, err)

	return flowgo.Event{
		Payload: ccfEvent,
	}
}

func jsonCDCEventFixture(t *testing.T) flowgo.Event {
	converted, err := convert.CcfEventToJsonEvent(ccfEventFixture(t))
	require.NoError(t, err)
	return *converted
}
