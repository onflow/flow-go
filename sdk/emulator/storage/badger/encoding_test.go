package badger

import (
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/sdk/emulator/types"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncodeTransaction(t *testing.T) {
	tx := unittest.TransactionFixture()
	data, err := encodeTransaction(tx)
	require.Nil(t, err)

	var decodedTx flow.Transaction
	err = decodeTransaction(&decodedTx, data)
	require.Nil(t, err)
	assert.Equal(t, tx, decodedTx)
}

func TestEncodeBlock(t *testing.T) {
	block := types.Block{
		Number:            1234,
		Timestamp:         time.Now(),
		PreviousBlockHash: unittest.HashFixture(32),
		TransactionHashes: []crypto.Hash{unittest.HashFixture(32)},
	}
	data, err := encodeBlock(block)
	require.Nil(t, err)

	var decodedBlock types.Block
	err = decodeBlock(&decodedBlock, data)
	require.Nil(t, err)

	assert.Equal(t, block.Number, decodedBlock.Number)
	assert.Equal(t, block.PreviousBlockHash, decodedBlock.PreviousBlockHash)
	assert.Equal(t, block.TransactionHashes, decodedBlock.TransactionHashes)
	// Compare timestamp using Equal because Gob encode/decode can cause struct
	// representation of time type to change.
	assert.True(t, block.Timestamp.Equal(decodedBlock.Timestamp))
}

func TestEncodeRegisters(t *testing.T) {
	registers := unittest.RegistersFixture()
	data, err := encodeRegisters(registers)
	require.Nil(t, err)

	var decodedRegisters flow.Registers
	err = decodeRegisters(&decodedRegisters, data)
	require.Nil(t, err)
	assert.Equal(t, registers, decodedRegisters)
}

func TestEncodeEventList(t *testing.T) {
	eventList := flow.EventList{unittest.EventFixture()}
	data, err := encodeEventList(eventList)
	require.Nil(t, err)

	var decodedEventList flow.EventList
	err = decodeEventList(&decodedEventList, data)
	require.Nil(t, err)
	assert.Equal(t, eventList, decodedEventList)
}
