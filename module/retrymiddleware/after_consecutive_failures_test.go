package retrymiddleware

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/sethvargo/go-retry"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/mock"
)

// TestAfterConsecutiveFailures test that the middleware executes the onConsecutiveFailures func as expected
func TestAfterConsecutiveFailures(t *testing.T) {
	a := &mock.DKGContractClient{}
	b := &mock.DKGContractClient{}
	c := &mock.DKGContractClient{}

	msg := messages.BroadcastDKGMessage{}

	a.On("Broadcast", msg).
		Return(errors.New("error from a")).
		Twice()

	b.On("Broadcast", msg).
		Return(errors.New("error from b")).
		Twice()

	c.On("Broadcast", msg).
		Return(errors.New("error from c")).
		Twice()

	clients := []*mock.DKGContractClient{a, b, c}

	// every 2 failures we will update our dkgContractClient
	maxConsecutiveRetries := 2

	expRetry, err := retry.NewConstant(1000 * time.Millisecond)
	if err != nil {
		require.NoError(t, err, "failed to create constant retry mechanism")
	}
	maxedExpRetry := retry.WithMaxRetries(5, expRetry)

	// after 2 consecutive failures fallback to next DKGContractClient
	clientIndex := 0
	dkgContractClient := clients[clientIndex]
	afterConsecutiveFailures := AfterConsecutiveFailures(maxConsecutiveRetries, maxedExpRetry, func(totalAttempts int) {
		if clientIndex == len(clients)-1 {
			clientIndex = 0
		} else {
			clientIndex++
		}
		dkgContractClient = clients[clientIndex]
	})

	err = retry.Do(context.Background(), afterConsecutiveFailures, func(ctx context.Context) error {
		err := dkgContractClient.Broadcast(msg)
		if err != nil {
			fmt.Printf("error: %s\n", err)
		}
		return retry.RetryableError(err)
	})
	require.Error(t, err)

	a.AssertExpectations(t)
	b.AssertExpectations(t)
	c.AssertExpectations(t)
}
