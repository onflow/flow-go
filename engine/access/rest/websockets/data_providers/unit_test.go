package data_providers

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rest/websockets/models"
	statestreamsmock "github.com/onflow/flow-go/engine/access/state_stream/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// testType represents a valid test scenario for subscribing
type testType struct {
	name              string
	arguments         models.Arguments
	setupBackend      func(sub *statestreamsmock.Subscription)
	expectedResponses []interface{}
}

// testErrType represents an error cases for subscribing
type testErrType struct {
	name             string
	arguments        models.Arguments
	expectedErrorMsg string
}

// testHappyPath tests a variety of scenarios for data providers in
// happy path scenarios. This function runs parameterized test cases that
// simulate various configurations and verifies that the data provider operates
// as expected without encountering errors.
//
// Arguments:
// - t: The testing context.
// - topic: The topic associated with the data provider.
// - factory: A factory for creating data provider instance.
// - tests: A slice of test cases to run, each specifying setup and validation logic.
// - sendData: A function to simulate emitting data into the subscription's data channel.
// - requireFn: A function to validate the output received in the send channel.
func testHappyPath(
	t *testing.T,
	topic string,
	factory *DataProviderFactoryImpl,
	tests []testType,
	sendData func(chan interface{}),
	requireFn func(interface{}, interface{}),
) {
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()
			send := make(chan interface{}, 10)

			// Create a channel to simulate the subscription's data channel
			dataChan := make(chan interface{})

			// Create a mock subscription and mock the channel
			sub := statestreamsmock.NewSubscription(t)
			sub.On("Channel").Return((<-chan interface{})(dataChan))
			sub.On("Err").Return(nil)
			test.setupBackend(sub)

			// Create the data provider instance
			provider, err := factory.NewDataProvider(ctx, "dummy-id", topic, test.arguments, send)
			require.NoError(t, err)
			require.NotNil(t, provider)

			// Ensure the provider is properly closed after the test
			defer provider.Close()

			// Run the provider in a separate goroutine
			done := make(chan struct{})
			go func() {
				defer close(done)
				err = provider.Run()
				require.NoError(t, err)
			}()

			// Simulate emitting data to the data channel
			go func() {
				defer close(dataChan)
				sendData(dataChan)
			}()

			// Wait for the provider goroutine to finish
			unittest.RequireCloseBefore(t, done, time.Second, "provider failed to stop")

			// Collect responses
			for i, expected := range test.expectedResponses {
				unittest.RequireReturnsBefore(t, func() {
					v, ok := <-send
					require.True(t, ok, "channel closed while waiting for response %v: err: %v", expected, sub.Err())

					requireFn(v, expected)
				}, time.Second, fmt.Sprintf("timed out waiting for response %d %v", i, expected))
			}
		})
	}
}

// extractPayload extracts the BaseDataProvidersResponse and its typed Payload.
func extractPayload[T any](t *testing.T, v interface{}) (*models.BaseDataProvidersResponse, T) {
	response, ok := v.(*models.BaseDataProvidersResponse)
	require.True(t, ok, "Expected *models.BaseDataProvidersResponse, got %T", v)

	payload, ok := response.Payload.(T)
	require.True(t, ok, "Unexpected response payload type: %T", response.Payload)

	return response, payload
}

func TestEnsureAllowedFields(t *testing.T) {
	t.Parallel()

	allowedFields := map[string]struct{}{
		"start_block_id":     {},
		"start_block_height": {},
		"event_types":        {},
		"account_addresses":  {},
		"heartbeat_interval": {},
	}

	t.Run("Valid fields with all required", func(t *testing.T) {
		fields := map[string]interface{}{
			"start_block_id":     "abc",
			"start_block_height": 123,
			"event_types":        []string{"flow.Event"},
			"account_addresses":  []string{"0x1"},
			"heartbeat_interval": 10,
		}
		if err := ensureAllowedFields(fields, allowedFields); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Unexpected field present", func(t *testing.T) {
		fields := map[string]interface{}{
			"start_block_id":     "abc",
			"start_block_height": 123,
			"unknown_field":      "unexpected",
		}
		if err := ensureAllowedFields(fields, allowedFields); err == nil {
			t.Error("expected error for unexpected field, got nil")
		}
	})
}

func TestExtractArrayOfStrings(t *testing.T) {
	tests := []struct {
		name      string
		args      models.Arguments
		key       string
		required  bool
		expect    []string
		expectErr bool
	}{
		{
			name:      "Valid string array",
			args:      models.Arguments{"tags": []string{"a", "b"}},
			key:       "tags",
			required:  true,
			expect:    []string{"a", "b"},
			expectErr: false,
		},
		{
			name:      "Missing required key",
			args:      models.Arguments{},
			key:       "tags",
			required:  true,
			expect:    nil,
			expectErr: true,
		},
		{
			name:      "Missing optional key",
			args:      models.Arguments{},
			key:       "tags",
			required:  false,
			expect:    []string{},
			expectErr: false,
		},
		{
			name:      "Invalid type in array",
			args:      models.Arguments{"tags": []interface{}{"a", 123}},
			key:       "tags",
			required:  true,
			expect:    nil,
			expectErr: true,
		},
		{
			name:      "Nil value",
			args:      models.Arguments{"tags": nil},
			key:       "tags",
			required:  false,
			expect:    nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractArrayOfStrings(tt.args, tt.key, tt.required)
			if (err != nil) != tt.expectErr {
				t.Fatalf("unexpected error status. got: %v, want error: %v", err, tt.expectErr)
			}
			if !reflect.DeepEqual(result, tt.expect) {
				t.Fatalf("unexpected result. got: %v, want: %v", result, tt.expect)
			}
		})
	}
}
