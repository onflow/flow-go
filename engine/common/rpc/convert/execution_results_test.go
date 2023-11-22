package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
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
	block.SetPayload(unittest.PayloadFixture(unittest.WithAllTheFixins))
	metaList := block.Payload.Receipts

	msg := convert.ExecutionResultMetaListToMessages(metaList)
	converted := convert.MessagesToExecutionResultMetaList(msg)

	assert.Equal(t, metaList, converted)
}
