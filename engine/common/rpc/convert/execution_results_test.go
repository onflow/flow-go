package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

func TestConvertExecutionResult(t *testing.T) {
	t.Parallel()

	er := unittest.ExecutionResultFixture(unittest.WithServiceEvents(3))

	msg, err := convert.ExecutionResultToMessage(er)
	require.NoError(t, err)

	converted, err := convert.MessageToExecutionResult(msg)
	require.NoError(t, err)

	assert.Equal(t, er, converted)
}

func TestConvertExecutionResults(t *testing.T) {
	t.Parallel()

	results := []*flow.ExecutionResult{
		unittest.ExecutionResultFixture(unittest.WithServiceEvents(3)),
		unittest.ExecutionResultFixture(unittest.WithServiceEvents(3)),
		unittest.ExecutionResultFixture(unittest.WithServiceEvents(3)),
	}

	msg, err := convert.ExecutionResultsToMessages(results)
	require.NoError(t, err)

	converted, err := convert.MessagesToExecutionResults(msg)
	require.NoError(t, err)

	assert.Equal(t, results, converted)
}

func TestConvertExecutionResultMetaList(t *testing.T) {
	t.Parallel()

	block := unittest.FullBlockFixture()
	metaList := block.Payload.Receipts

	msg := convert.ExecutionResultMetaListToMessages(metaList)
	converted, err := convert.MessagesToExecutionResultMetaList(msg)
	require.NoError(t, err)

	assert.Equal(t, metaList, converted)
}

// TestConvertExecutionReceiptExcludeResult tests converting an execution receipt to a proto message
// with includeResult=false. Meta fields must match the original receipt and ExecutionResult must be nil.
func TestConvertExecutionReceiptExcludeResult(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()
	receipt := g.ExecutionReceipts().Fixture()

	msg, err := convert.ExecutionReceiptToMessage(receipt, false)
	require.NoError(t, err)
	require.NotNil(t, msg.Meta)
	assert.Nil(t, msg.ExecutionResult)

	assert.Equal(t, receipt.ExecutorID, convert.MessageToIdentifier(msg.Meta.ExecutorId))
	assert.Equal(t, receipt.ExecutionResult.ID(), convert.MessageToIdentifier(msg.Meta.ResultId))
	assert.Equal(t, receipt.ExecutorSignature, convert.MessageToSignature(msg.Meta.ExecutorSignature))
	assert.Equal(t, receipt.Spocks, convert.MessagesToSignatures(msg.Meta.Spocks))
}

// TestConvertExecutionReceiptIncludeResult tests converting an execution receipt to a proto message
// with includeResult=true. Meta fields must match and ExecutionResult must round-trip correctly.
func TestConvertExecutionReceiptIncludeResult(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()
	receipt := g.ExecutionReceipts().Fixture()

	msg, err := convert.ExecutionReceiptToMessage(receipt, true)
	require.NoError(t, err)
	require.NotNil(t, msg.Meta)
	require.NotNil(t, msg.ExecutionResult)

	assert.Equal(t, receipt.ExecutorID, convert.MessageToIdentifier(msg.Meta.ExecutorId))
	assert.Equal(t, receipt.ExecutionResult.ID(), convert.MessageToIdentifier(msg.Meta.ResultId))
	assert.Equal(t, receipt.ExecutorSignature, convert.MessageToSignature(msg.Meta.ExecutorSignature))
	assert.Equal(t, receipt.Spocks, convert.MessagesToSignatures(msg.Meta.Spocks))

	convertedResult, err := convert.MessageToExecutionResult(msg.ExecutionResult)
	require.NoError(t, err)
	assert.Equal(t, &receipt.ExecutionResult, convertedResult)
}

// TestConvertExecutionReceiptsExcludeResult tests the batch conversion with includeResult=false.
func TestConvertExecutionReceiptsExcludeResult(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()
	receipts := g.ExecutionReceipts().List(3)

	msgs, err := convert.ExecutionReceiptsToMessages(receipts, false)
	require.NoError(t, err)
	require.Len(t, msgs, len(receipts))

	for i, msg := range msgs {
		require.NotNil(t, msg.Meta)
		assert.Nil(t, msg.ExecutionResult)
		assert.Equal(t, receipts[i].ExecutorID, convert.MessageToIdentifier(msg.Meta.ExecutorId))
		assert.Equal(t, receipts[i].ExecutionResult.ID(), convert.MessageToIdentifier(msg.Meta.ResultId))
		assert.Equal(t, receipts[i].ExecutorSignature, convert.MessageToSignature(msg.Meta.ExecutorSignature))
		assert.Equal(t, receipts[i].Spocks, convert.MessagesToSignatures(msg.Meta.Spocks))
	}
}

// TestConvertExecutionReceiptsIncludeResult tests the batch conversion with includeResult=true.
func TestConvertExecutionReceiptsIncludeResult(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()
	receipts := g.ExecutionReceipts().List(3)

	msgs, err := convert.ExecutionReceiptsToMessages(receipts, true)
	require.NoError(t, err)
	require.Len(t, msgs, len(receipts))

	for i, msg := range msgs {
		require.NotNil(t, msg.Meta)
		require.NotNil(t, msg.ExecutionResult)
		assert.Equal(t, receipts[i].ExecutorID, convert.MessageToIdentifier(msg.Meta.ExecutorId))
		assert.Equal(t, receipts[i].ExecutionResult.ID(), convert.MessageToIdentifier(msg.Meta.ResultId))
		assert.Equal(t, receipts[i].ExecutorSignature, convert.MessageToSignature(msg.Meta.ExecutorSignature))
		assert.Equal(t, receipts[i].Spocks, convert.MessagesToSignatures(msg.Meta.Spocks))

		convertedResult, err := convert.MessageToExecutionResult(msg.ExecutionResult)
		require.NoError(t, err)
		assert.Equal(t, &receipts[i].ExecutionResult, convertedResult)
	}
}
