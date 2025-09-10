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

func TestMessageToExecutorMetadata_Populated(t *testing.T) {
	erID := unittest.IdentifierFixture()
	exec1 := unittest.IdentifierFixture()
	exec2 := unittest.IdentifierFixture()

	msg := &entities.ExecutorMetadata{
		ExecutionResultId: erID[:],
		ExecutorIds:       [][]byte{exec1[:], exec2[:]},
	}

	actual := convert.MessageToExecutorMetadata(msg)
	require.NotNil(t, actual)
	require.Equal(t, erID, actual.ExecutionResultID)
	require.Len(t, actual.ExecutorIDs, 2)
	require.Equal(t, exec1, actual.ExecutorIDs[0])
	require.Equal(t, exec2, actual.ExecutorIDs[1])
}

func TestMessageToExecutorMetadata_Nil(t *testing.T) {
	actual := convert.MessageToExecutorMetadata(nil)
	require.Nil(t, actual)
}

func TestMessageToExecutorMetadata_Empty(t *testing.T) {
	actual := convert.MessageToExecutorMetadata(&entities.ExecutorMetadata{})
	require.NotNil(t, actual)
	require.Equal(t, flow.ZeroID, actual.ExecutionResultID)
	require.Empty(t, actual.ExecutorIDs)
}

func TestExecutorMetadataToMessage_Populated(t *testing.T) {
	erID := unittest.IdentifierFixture()
	exec1 := unittest.IdentifierFixture()
	exec2 := unittest.IdentifierFixture()

	in := &access.ExecutorMetadata{
		ExecutionResultID: erID,
		ExecutorIDs:       []flow.Identifier{exec1, exec2},
	}

	msg := convert.ExecutorMetadataToMessage(in)
	require.NotNil(t, msg)
	require.Equal(t, erID[:], msg.ExecutionResultId)
	require.Len(t, msg.ExecutorIds, 2)
	require.Equal(t, exec1[:], msg.ExecutorIds[0])
	require.Equal(t, exec2[:], msg.ExecutorIds[1])
}

func TestExecutorMetadataToMessage_Empty(t *testing.T) {
	msg := convert.ExecutorMetadataToMessage(&access.ExecutorMetadata{})
	require.NotNil(t, msg)
	require.Equal(t, flow.ZeroID[:], msg.ExecutionResultId)
	require.Empty(t, msg.ExecutorIds)
}

func TestExecutorMetadataToMessage_Nil(t *testing.T) {
	msg := convert.ExecutorMetadataToMessage(nil)
	require.Nil(t, msg)
}
