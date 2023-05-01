package convert_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-go/model/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEventConversion(t *testing.T) {

	chainID := flow.Emulator

	t.Run(
		"epoch setup", func(t *testing.T) {

			fixture, expected := unittest.EpochSetupFixtureByChainID(chainID)

			// convert Cadence types to Go types
			event, err := convert.ServiceEvent(chainID, fixture)
			require.NoError(t, err)
			require.NotNil(t, event)

			// cast event type to epoch setup
			actual, ok := event.Event.(*flow.EpochSetup)
			require.True(t, ok)

			assert.Equal(t, expected, actual)

		},
	)

	t.Run(
		"epoch commit", func(t *testing.T) {

			fixture, expected := unittest.EpochCommitFixtureByChainID(chainID)

			// convert Cadence types to Go types
			event, err := convert.ServiceEvent(chainID, fixture)
			require.NoError(t, err)
			require.NotNil(t, event)

			// cast event type to epoch commit
			actual, ok := event.Event.(*flow.EpochCommit)
			require.True(t, ok)

			assert.Equal(t, expected, actual)
		},
	)

	t.Run(
		"version beacon", func(t *testing.T) {

			fixture, expected := unittest.VersionBeaconFixtureByChainID(chainID)

			// convert Cadence types to Go types
			event, err := convert.ServiceEvent(chainID, fixture)
			require.NoError(t, err)
			require.NotNil(t, event)

			// cast event type to version beacon
			actual, ok := event.Event.(*flow.VersionBeacon)
			require.True(t, ok)

			assert.Equal(t, expected, actual)
		},
	)
}

func TestDecodeCadenceValue(t *testing.T) {

	tests := []struct {
		name             string
		location         string
		value            cadence.Value
		decodeInner      func(cadence.Value) (interface{}, error)
		expected         interface{}
		expectError      bool
		expectedLocation string
	}{
		{
			name:     "Basic",
			location: "test",
			value:    cadence.UInt64(42),
			decodeInner: func(value cadence.Value) (
				interface{},
				error,
			) {
				return 42, nil
			},
			expected:    42,
			expectError: false,
		},
		{
			name:     "Nil value",
			location: "test",
			value:    nil,
			decodeInner: func(value cadence.Value) (
				interface{},
				error,
			) {
				return 42, nil
			},
			expected:    nil,
			expectError: true,
		},
		{
			name:     "Custom decode error",
			location: "test",
			value:    cadence.String("hello"),
			decodeInner: func(value cadence.Value) (
				interface{},
				error,
			) {
				return nil, fmt.Errorf("custom error")
			},
			expected:    nil,
			expectError: true,
		},
		{
			name:     "Nested location",
			location: "outer",
			value:    cadence.String("hello"),
			decodeInner: func(value cadence.Value) (interface{}, error) {
				return convert.DecodeCadenceValue(
					".inner", value,
					func(value cadence.Value) (interface{}, error) {
						return nil, fmt.Errorf("custom error")
					},
				)
			},
			expected:         nil,
			expectError:      true,
			expectedLocation: "outer.inner",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result, err := convert.DecodeCadenceValue(
					tt.location,
					tt.value,
					tt.decodeInner,
				)

				if tt.expectError {
					assert.Error(t, err)
					if tt.expectedLocation != "" {
						assert.Contains(t, err.Error(), tt.expectedLocation)
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.expected, result)
				}
			},
		)
	}
}
