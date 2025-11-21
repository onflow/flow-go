package factory

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectionSyncMode_String tests the String method for all CollectionSyncMode constants.
func TestCollectionSyncMode_String(t *testing.T) {
	tests := []struct {
		name     string
		mode     CollectionSyncMode
		expected string
	}{
		{
			name:     "execution_first",
			mode:     CollectionSyncModeExecutionFirst,
			expected: "execution_first",
		},
		{
			name:     "execution_and_collection",
			mode:     CollectionSyncModeExecutionAndCollection,
			expected: "execution_and_collection",
		},
		{
			name:     "collection_only",
			mode:     CollectionSyncModeCollectionOnly,
			expected: "collection_only",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, test.mode.String())
		})
	}
}

// TestParseCollectionSyncMode_Valid tests the ParseCollectionSyncMode function with valid inputs.
func TestParseCollectionSyncMode_Valid(t *testing.T) {
	tests := map[string]CollectionSyncMode{
		"execution_first":          CollectionSyncModeExecutionFirst,
		"execution_and_collection": CollectionSyncModeExecutionAndCollection,
		"collection_only":          CollectionSyncModeCollectionOnly,
	}

	for input, expectedMode := range tests {
		t.Run(input, func(t *testing.T) {
			mode, err := ParseCollectionSyncMode(input)
			require.NoError(t, err)
			assert.Equal(t, expectedMode, mode)
		})
	}
}

// TestParseCollectionSyncMode_Invalid tests the ParseCollectionSyncMode function with invalid inputs.
func TestParseCollectionSyncMode_Invalid(t *testing.T) {
	invalidInputs := []string{
		"",
		"unknown",
		"invalid",
		"execution-first",                // wrong separator
		"executionFirst",                 // wrong format
		"EXECUTION_FIRST",                // wrong case
		"execution_and_collection_extra", // extra suffix
	}

	for _, input := range invalidInputs {
		t.Run(input, func(t *testing.T) {
			mode, err := ParseCollectionSyncMode(input)
			assert.Error(t, err)
			assert.Empty(t, mode)
			expectedErr := fmt.Errorf("invalid collection sync mode: %s, must be one of [execution_first, execution_and_collection, collection_only]", input)
			assert.EqualError(t, err, expectedErr.Error())
		})
	}
}

// TestCollectionSyncMode_ShouldCreateFetcher tests the ShouldCreateFetcher method with all combinations
// of modes and execution data sync enabled/disabled.
func TestCollectionSyncMode_ShouldCreateFetcher(t *testing.T) {
	tests := []struct {
		name                     string
		mode                     CollectionSyncMode
		executionDataSyncEnabled bool
		expectedShouldCreate     bool
		description              string
	}{
		{
			name:                     "execution_first_with_execution_data_sync_enabled",
			mode:                     CollectionSyncModeExecutionFirst,
			executionDataSyncEnabled: true,
			expectedShouldCreate:     false,
			description:              "should not create fetcher when execution data sync is enabled",
		},
		{
			name:                     "execution_first_with_execution_data_sync_disabled",
			mode:                     CollectionSyncModeExecutionFirst,
			executionDataSyncEnabled: false,
			expectedShouldCreate:     true,
			description:              "should create fetcher when execution data sync is disabled",
		},
		{
			name:                     "execution_and_collection_with_execution_data_sync_enabled",
			mode:                     CollectionSyncModeExecutionAndCollection,
			executionDataSyncEnabled: true,
			expectedShouldCreate:     true,
			description:              "should always create fetcher for execution_and_collection mode",
		},
		{
			name:                     "execution_and_collection_with_execution_data_sync_disabled",
			mode:                     CollectionSyncModeExecutionAndCollection,
			executionDataSyncEnabled: false,
			expectedShouldCreate:     true,
			description:              "should always create fetcher for execution_and_collection mode",
		},
		{
			name:                     "collection_only_with_execution_data_sync_enabled",
			mode:                     CollectionSyncModeCollectionOnly,
			executionDataSyncEnabled: true,
			expectedShouldCreate:     true,
			description:              "should always create fetcher for collection_only mode",
		},
		{
			name:                     "collection_only_with_execution_data_sync_disabled",
			mode:                     CollectionSyncModeCollectionOnly,
			executionDataSyncEnabled: false,
			expectedShouldCreate:     true,
			description:              "should always create fetcher for collection_only mode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.mode.ShouldCreateFetcher(test.executionDataSyncEnabled)
			assert.Equal(t, test.expectedShouldCreate, result, test.description)
		})
	}
}

// TestCollectionSyncMode_ShouldCreateExecutionDataProcessor tests the ShouldCreateExecutionDataProcessor method
// with all combinations of modes and execution data sync enabled/disabled.
func TestCollectionSyncMode_ShouldCreateExecutionDataProcessor(t *testing.T) {
	tests := []struct {
		name                     string
		mode                     CollectionSyncMode
		executionDataSyncEnabled bool
		expectedShouldCreate     bool
		description              string
	}{
		{
			name:                     "execution_first_with_execution_data_sync_enabled",
			mode:                     CollectionSyncModeExecutionFirst,
			executionDataSyncEnabled: true,
			expectedShouldCreate:     true,
			description:              "should create processor when execution data sync is enabled",
		},
		{
			name:                     "execution_first_with_execution_data_sync_disabled",
			mode:                     CollectionSyncModeExecutionFirst,
			executionDataSyncEnabled: false,
			expectedShouldCreate:     false,
			description:              "should not create processor when execution data sync is disabled",
		},
		{
			name:                     "execution_and_collection_with_execution_data_sync_enabled",
			mode:                     CollectionSyncModeExecutionAndCollection,
			executionDataSyncEnabled: true,
			expectedShouldCreate:     true,
			description:              "should create processor when execution data sync is enabled",
		},
		{
			name:                     "execution_and_collection_with_execution_data_sync_disabled",
			mode:                     CollectionSyncModeExecutionAndCollection,
			executionDataSyncEnabled: false,
			expectedShouldCreate:     false,
			description:              "should not create processor when execution data sync is disabled",
		},
		{
			name:                     "collection_only_with_execution_data_sync_enabled",
			mode:                     CollectionSyncModeCollectionOnly,
			executionDataSyncEnabled: true,
			expectedShouldCreate:     false,
			description:              "should not create processor for collection_only mode even with execution data sync enabled",
		},
		{
			name:                     "collection_only_with_execution_data_sync_disabled",
			mode:                     CollectionSyncModeCollectionOnly,
			executionDataSyncEnabled: false,
			expectedShouldCreate:     false,
			description:              "should not create processor for collection_only mode",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.mode.ShouldCreateExecutionDataProcessor(test.executionDataSyncEnabled)
			assert.Equal(t, test.expectedShouldCreate, result, test.description)
		})
	}
}
