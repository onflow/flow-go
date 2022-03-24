package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertTransaction(t *testing.T) {
	tx := unittest.TransactionBodyFixture()

	msg := convert.TransactionToMessage(tx)
	converted, err := convert.MessageToTransaction(msg, flow.Testnet.Chain())
	assert.NoError(t, err)

	assert.Equal(t, tx, converted)
	assert.Equal(t, tx.ID(), converted.ID())
}

func TestConvertAccountKey(t *testing.T) {
	privateKey, _ := unittest.AccountKeyDefaultFixture()
	accountKey := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)

	// Explicitly test if Revoked is properly converted
	accountKey.Revoked = true

	msg, err := convert.AccountKeyToMessage(accountKey)
	assert.NoError(t, err)

	converted, err := convert.MessageToAccountKey(msg)
	assert.NoError(t, err)

	assert.Equal(t, accountKey, *converted)
	assert.Equal(t, accountKey.PublicKey, converted.PublicKey)
	assert.Equal(t, accountKey.Revoked, converted.Revoked)
}

func TestConvertFullBlock(t *testing.T) {
	fullBlock := unittest.FullBlockFixture()

	msg, err := convert.BlockToMessage(&fullBlock)
	assert.NoError(t, err)

	// extract payload for payload hash check
	extractedPayload, err := convert.PayloadFromMessage(msg)
	assert.NoError(t, err)

	converted, err := convert.MessageToBlock(msg)
	assert.NoError(t, err)

	assert.Equal(t, converted.ID(), fullBlock.ID())
	assert.Equal(t, converted.Payload.Hash(), fullBlock.Payload.Hash())
	assert.Equal(t, extractedPayload.Hash(), fullBlock.Payload.Hash())
}

func TestConvertBlockHeader(t *testing.T) {
	header := unittest.BlockHeaderFixture()

	msg, err := convert.BlockHeaderToMessage(&header)
	assert.NoError(t, err)

	converted, err := convert.MessageToBlockHeader(msg)
	assert.NoError(t, err)

	assert.Equal(t, converted.ID(), header.ID())
}

func TestConvertSeal(t *testing.T) {
	blockSeals := unittest.BlockSealsFixture(3)

	msg := convert.BlockSealsToMessages(blockSeals)

	converted, err := convert.MessagesToBlockSeals(msg)
	assert.NoError(t, err)

	for idx, convertedSeal := range converted {
		assert.Equal(t, convertedSeal.ID(), blockSeals[idx].ID())
	}
}

func TestConvertEvent(t *testing.T) {
	txId := unittest.TransactionFixture().ID()
	setupEvent := unittest.EventFixture(flow.ServiceEventSetup, uint32(1), uint32(1), txId, 0)

	msg := convert.EventToMessage(setupEvent)
	converted := convert.MessageToEvent(msg)
	assert.Equal(t, setupEvent.ID(), converted.ID())

	commitEvent := unittest.EventFixture(flow.ServiceEventCommit, uint32(1), uint32(1), txId, 0)

	msg = convert.EventToMessage(commitEvent)
	converted = convert.MessageToEvent(msg)
	assert.Equal(t, setupEvent.ID(), converted.ID())
}

func TestConvertCollectionGuarantee(t *testing.T) {
	collectionGuarantee := unittest.CollectionGuaranteeFixture()

	msg := convert.CollectionGuaranteeToMessage(collectionGuarantee)

	converted := convert.MessageToCollectionGuarantee(msg)

	assert.Equal(t, collectionGuarantee.ID(), converted.ID())
}

func TestConvertExecutionResult(t *testing.T) {
	execResult := unittest.ExecutionResultFixture()

	msg, err := convert.ExecutionResultToMessage(execResult)
	assert.NoError(t, err)

	converted, err := convert.MessageToExecutionResult(msg)
	assert.NoError(t, err)

	assert.Equal(t, execResult.ID(), converted.ID())
}

func TestConvertChunk(t *testing.T) {
	blockId := unittest.BlockFixture().ID()
	collectionIndex := uint(0)
	chunk := unittest.ChunkFixture(blockId, collectionIndex)

	msg := convert.ChunkToMessage(chunk)

	converted, err := convert.MessageToChunk(msg)

	assert.NoError(t, err)
	assert.Equal(t, chunk.ID(), converted.ID())
}

func TestConvertIdentifier(t *testing.T) {
	id := unittest.IdentifierFixture()

	msg := convert.IdentifierToMessage(id)

	converted := convert.MessageToIdentifier(msg)
	assert.Equal(t, id, converted)
}
