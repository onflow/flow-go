package parser

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewExecutionDataQuery(t *testing.T) {
	validIDs := unittest.IdentifierListFixture(5)

	tests := []struct {
		name                  string
		agreeingExecutorCount string
		requiredExecutorIds   []string
		includeExecutorMeta   string
		wantErr               bool
	}{
		{
			name:                  "valid input",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   validIDs.Strings(),
			includeExecutorMeta:   "true",
			wantErr:               false,
		},
		{
			name:                  "valid empty input",
			agreeingExecutorCount: "",
			requiredExecutorIds:   []string{},
			includeExecutorMeta:   "",
			wantErr:               false,
		},
		{
			name:                  "invalid agreeingExecutorCount (not a number)",
			agreeingExecutorCount: "not-a-number",
			requiredExecutorIds:   validIDs.Strings(),
			includeExecutorMeta:   "true",
			wantErr:               true,
		},
		{
			name:                  "invalid agreeingExecutorCount (count equals zero)",
			agreeingExecutorCount: "0",
			requiredExecutorIds:   validIDs.Strings(),
			includeExecutorMeta:   "true",
			wantErr:               true,
		},
		{
			name:                  "invalid requiredExecutorIds",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   []string{"1234"},
			includeExecutorMeta:   "true",
			wantErr:               true,
		},
		{
			name:                  "too many requiredExecutorIds",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   make([]string, MaxIDsLength+1),
			includeExecutorMeta:   "true",
			wantErr:               true,
		},
		{
			name:                  "invalid includeExecutorMetadata",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   validIDs.Strings(),
			includeExecutorMeta:   "notabool",
			wantErr:               true,
		},
		{
			name:                  "duplicate requiredExecutorIds",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   []string{validIDs[0].String(), validIDs[0].String()},
			includeExecutorMeta:   "false",
			wantErr:               false,
		},
		{
			name:                  "empty requiredExecutorIds allowed",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   []string{},
			includeExecutorMeta:   "false",
			wantErr:               false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count, ids, include, err := NewExecutionDataQuery(
				tt.agreeingExecutorCount,
				tt.requiredExecutorIds,
				tt.includeExecutorMeta,
			)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.GreaterOrEqual(t, count, uint64(0))
			require.NotNil(t, ids)
			require.IsType(t, [][]byte{}, ids)
			require.IsType(t, true, include)
		})
	}
}
