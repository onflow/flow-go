package convert_test

import (
	"testing"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExecutorMetadata(t *testing.T) {
	execResultID := unittest.IdentifierFixture()
	exec1 := unittest.IdentifierFixture()
	exec2 := unittest.IdentifierFixture()

	originalMessage := entities.ExecutorMetadata{
		ExecutionResultId: execResultID[:],
		ExecutorId:        [][]byte{exec1[:], exec2[:]},
	}

	// 1. convert message to metadata
	actual := convert.MessageToExecutorMetadata(originalMessage)
	require.NotEmpty(t, actual)
	require.Equal(t, execResultID, actual.ExecutionResultID)
	require.Len(t, actual.ExecutorIDs, 2)
	require.Equal(t, exec1, actual.ExecutorIDs[0])
	require.Equal(t, exec2, actual.ExecutorIDs[1])

	// 2. convert metadata back to message
	message := convert.ExecutorMetadataToMessage(actual)
	require.Equal(t, originalMessage, message)
}

func TestMessageToExecutorMetadata_Empty(t *testing.T) {
	actual := convert.MessageToExecutorMetadata(entities.ExecutorMetadata{})
	require.NotNil(t, actual)
	require.Equal(t, flow.ZeroID, actual.ExecutionResultID)
	require.Empty(t, actual.ExecutorIDs)
}

func TestExecutorMetadataToMessage_Empty(t *testing.T) {
	msg := convert.ExecutorMetadataToMessage(access.ExecutorMetadata{})
	require.NotNil(t, msg)
	require.Equal(t, flow.ZeroID[:], msg.ExecutionResultId)
	require.Empty(t, msg.ExecutorId)
}
