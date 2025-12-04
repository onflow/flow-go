package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rest/http/models"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestNewExecutionDataQuery(t *testing.T) {
	validIDs := unittest.IdentifierListFixture(5)

	tests := []struct {
		name                  string
		agreeingExecutorCount string
		requiredExecutorIds   []string
		includeExecutorMeta   string
		expected              *models.ExecutionStateQuery
		wantErr               bool
	}{
		{
			name:                  "valid input",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   validIDs.Strings(),
			includeExecutorMeta:   "true",
			expected: &models.ExecutionStateQuery{
				AgreeingExecutorsCount:  5,
				RequiredExecutorIDs:     validIDs,
				IncludeExecutorMetadata: true,
			},
		},
		{
			name:                  "valid empty input",
			agreeingExecutorCount: "",
			requiredExecutorIds:   []string{},
			includeExecutorMeta:   "",
			expected: &models.ExecutionStateQuery{
				AgreeingExecutorsCount:  0,
				RequiredExecutorIDs:     nil,
				IncludeExecutorMetadata: false,
			},
		},
		{
			name:                  "duplicate requiredExecutorIds",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   []string{validIDs[0].String(), validIDs[0].String()},
			includeExecutorMeta:   "false",
			expected: &models.ExecutionStateQuery{
				AgreeingExecutorsCount:  5,
				RequiredExecutorIDs:     []flow.Identifier{validIDs[0]},
				IncludeExecutorMetadata: false,
			},
		},
		{
			name:                  "empty requiredExecutorIds allowed",
			agreeingExecutorCount: "5",
			requiredExecutorIds:   []string{},
			includeExecutorMeta:   "false",
			expected: &models.ExecutionStateQuery{
				AgreeingExecutorsCount:  5,
				RequiredExecutorIDs:     nil,
				IncludeExecutorMetadata: false,
			},
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query, err := NewExecutionStateQuery(
				tt.agreeingExecutorCount,
				tt.requiredExecutorIds,
				tt.includeExecutorMeta,
			)

			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.expected, query)
		})
	}
}
