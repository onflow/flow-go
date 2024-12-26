package data_providers

import (
	"context"
	"fmt"
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
			provider, err := factory.NewDataProvider(ctx, topic, test.arguments, send)
			require.NotNil(t, provider)
			require.NoError(t, err)

			// Run the provider in a separate goroutine
			go func() {
				err = provider.Run()
				require.NoError(t, err)
			}()

			// Simulate emitting data to the data channel
			go func() {
				defer close(dataChan)
				sendData(dataChan)
			}()

			// Collect responses
			for i, expected := range test.expectedResponses {
				unittest.RequireReturnsBefore(t, func() {
					v, ok := <-send
					require.True(t, ok, "channel closed while waiting for response %v: err: %v", expected, sub.Err())

					requireFn(v, expected)
				}, time.Second, fmt.Sprintf("timed out waiting for response %d %v", i, expected))
			}

			// Ensure the provider is properly closed after the test
			provider.Close()
		})
	}
}
