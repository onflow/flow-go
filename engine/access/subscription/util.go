package subscription

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"

	"github.com/onflow/flow-go/engine/common/rpc"
)

// HandleSubscription is a generic handler for subscriptions to a specific type. It continuously listens to the subscription channel,
// handles the received responses, and sends the processed information to the client via the provided stream using handleResponse.
//
// Parameters:
// - ctx: Context for the operation.
// - sub: The subscription.
// - handleResponse: The function responsible for handling the response of the subscribed type.
//
// No errors are expected during normal operations.
func HandleSubscription[T any](ctx context.Context, sub Subscription, handleResponse func(resp T) error) error {
	for {
		select {
		case v, ok := <-sub.Channel():
			if !ok {
				if sub.Err() != nil {
					return fmt.Errorf("stream encountered an error: %w", sub.Err())
				}
				return nil
			}

			resp, ok := v.(T)
			if !ok {
				return fmt.Errorf("unexpected response type: %T", v)
			}

			err := handleResponse(resp)
			if err != nil {
				return err
			}
		case <-ctx.Done():
			return nil
		}
	}
}

// HandleRPCSubscription is a generic handler for subscriptions to a specific type for rpc calls.
//
// Parameters:
// - sub: The subscription.
// - handleResponse: The function responsible for handling the response of the subscribed type.
//
// Expected errors during normal operation:
//   - codes.Internal: If the subscription encounters an error or gets an unexpected response.
func HandleRPCSubscription[T any](sub Subscription, handleResponse func(resp T) error) error {
	err := HandleSubscription(context.TODO(), sub, handleResponse)
	if err != nil {
		return rpc.ConvertError(err, "handle subscription error", codes.Internal)
	}

	return nil
}
