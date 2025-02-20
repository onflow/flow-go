package convert_test

import (
	"math"
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

// TestServiceEventCount_conversion verifies that encoding `ServiceEventCount` = nil
// into uint32 works as expected.
// TODO(mainnet27, #6773): remove this function
func TestServiceEventCountFieldConversion(t *testing.T) {
	for i := uint16(0); i < 20; i++ {
		j := convert.ServiceEventCountFieldToMessage(&i)
		r := convert.MessageToServiceEventCountField(j)
		require.NotNil(t, r, "encoding value %d resulted in nil", i)
		require.Equal(t, i, *r)
	}
	for i := uint16(math.MaxUint16); i > math.MaxUint16-20; i-- {
		j := convert.ServiceEventCountFieldToMessage(&i)
		r := convert.MessageToServiceEventCountField(j)
		require.NotNil(t, r, "encoding value %d resulted in nil", i)
		require.Equal(t, i, *r)
	}
	// beyond the range of uint16, uint32 values should be decoded as nil
	for _, j := range []uint32{uint32(math.MaxUint16 + 1), uint32(math.MaxUint32)} {
		r := convert.MessageToServiceEventCountField(j)
		require.Nil(t, r)
	}
}

// Tests that Protobuf conversion is reversible for old chunk format (Protocol Version <2)
// TODO(mainnet27, #6773): remove this function
func TestConvertExecutionResult_ProtocolVersion1(t *testing.T) {
	t.Parallel()

	er := unittest.ExecutionResultFixture(unittest.WithServiceEvents(3))
	for _, chunk := range er.Chunks {
		chunk.ServiceEventCount = nil
	}

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
