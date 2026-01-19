package data_providers

import (
	"testing"

	"github.com/stretchr/testify/require"

	httpmodels "github.com/onflow/flow-go/engine/access/rest/http/models"
	wsmodels "github.com/onflow/flow-go/engine/access/rest/websockets/models"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestExtractHeartbeatInterval(t *testing.T) {
	defaultValue := uint64(42)

	// missing -> default
	v, err := extractHeartbeatInterval(wsmodels.Arguments{}, defaultValue)
	require.NoError(t, err)
	require.Equal(t, defaultValue, v)

	// bad type -> ErrInvalidArgument
	_, err = extractHeartbeatInterval(wsmodels.Arguments{"heartbeat_interval": 7}, defaultValue)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)

	// bad parse -> ErrInvalidArgument
	_, err = extractHeartbeatInterval(wsmodels.Arguments{"heartbeat_interval": "NaN"}, defaultValue)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)

	// ok
	v, err = extractHeartbeatInterval(wsmodels.Arguments{"heartbeat_interval": "100"}, defaultValue)
	require.NoError(t, err)
	require.EqualValues(t, 100, v)
}

func TestExtractExecutionStateQueryFields(t *testing.T) {
	// required missing -> ErrInvalidArgument
	_, err := extractExecutionStateQueryFields(
		wsmodels.Arguments{},
		"execution_state_query",
		true,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)

	// not a map -> ErrInvalidArgument
	_, err = extractExecutionStateQueryFields(
		wsmodels.Arguments{"execution_state_query": 1},
		"execution_state_query",
		false,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)

	// valid values
	query, err := extractExecutionStateQueryFields(
		wsmodels.Arguments{
			"execution_state_query": map[string]interface{}{
				"agreeing_executors_count": "3",
				"required_executor_ids": []interface{}{
					unittest.IdentifierFixture().String(),
					unittest.IdentifierFixture().String(),
				},
				"include_executor_metadata": "true",
			},
		},
		"execution_state_query",
		false,
	)
	require.NoError(t, err)
	require.EqualValues(t, 3, query.AgreeingExecutorsCount)
	require.True(t, query.IncludeExecutorMetadata)

	// ensure type matches expected model type
	var _ httpmodels.ExecutionStateQuery = query

	// invalid agreeing_executors_count type
	_, err = extractExecutionStateQueryFields(
		wsmodels.Arguments{
			"execution_state_query": map[string]interface{}{
				"agreeing_executors_count": 3, // not string
			},
		}, "execution_state_query", false,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)

	// invalid include_executor_metadata type
	_, err = extractExecutionStateQueryFields(
		wsmodels.Arguments{
			"execution_state_query": map[string]interface{}{
				"include_executor_metadata": true, // not string
			},
		},
		"execution_state_query",
		false,
	)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrInvalidArgument)
}
