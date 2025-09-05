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
		ExecutorId:        [][]byte{exec1[:], exec2[:]},
	}

	actual, err := convert.MessageToExecutorMetadata(msg)
	require.NoError(t, err)
	require.NotNil(t, actual)
	require.Equal(t, erID, actual.ExecutionResultID)
	require.Len(t, actual.ExecutorIDs, 2)
	require.Equal(t, exec1, actual.ExecutorIDs[0])
	require.Equal(t, exec2, actual.ExecutorIDs[1])
}

func TestMessageToExecutorMetadata_Nil(t *testing.T) {
	actual, err := convert.MessageToExecutorMetadata(nil)
	require.ErrorIs(t, err, convert.ErrEmptyMessage)
	require.Nil(t, actual)
}

func TestMessageToExecutorMetadata_Empty(t *testing.T) {
	actual, err := convert.MessageToExecutorMetadata(&entities.ExecutorMetadata{})
	require.NoError(t, err)
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

	msg, err := convert.ExecutorMetadataToMessage(in)
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Equal(t, erID[:], msg.ExecutionResultId)
	require.Len(t, msg.ExecutorId, 2)
	require.Equal(t, exec1[:], msg.ExecutorId[0])
	require.Equal(t, exec2[:], msg.ExecutorId[1])
}

func TestExecutorMetadataToMessage_Empty(t *testing.T) {
	msg, err := convert.ExecutorMetadataToMessage(&access.ExecutorMetadata{})
	require.NoError(t, err)
	require.NotNil(t, msg)
	require.Equal(t, flow.ZeroID[:], msg.ExecutionResultId)
	require.Empty(t, msg.ExecutorId)
}

func TestExecutorMetadataToMessage_Nil(t *testing.T) {
	msg, err := convert.ExecutorMetadataToMessage(nil)
	require.ErrorIs(t, err, convert.ErrEmptyMessage)
	require.Nil(t, msg)
}
