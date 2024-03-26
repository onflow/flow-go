package subscription

import (
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
)

// HandleSubscription is a generic handler for subscriptions to a specific type. It continuously listens to the subscription channel,
// handles the received responses, and sends the processed information to the client via the provided stream using handleResponse.
//
// Parameters:
// - sub: The subscription.
// - handleResponse: The function responsible for handling the response of the subscribed type.
//
// Expected errors during normal operation:
//   - codes.Internal: If the subscription encounters an error or gets an unexpected response.
func HandleSubscription[T any](sub Subscription, handleResponse func(resp T) error) error {
	for {
		v, ok := <-sub.Channel()
		if !ok {
			if sub.Err() != nil {
				return rpc.ConvertError(sub.Err(), "stream encountered an error", codes.Internal)
			}
			return nil
		}

		resp, ok := v.(T)
		if !ok {
			return status.Errorf(codes.Internal, "unexpected response type: %T", v)
		}

		err := handleResponse(resp)
		if err != nil {
			return err
		}
	}
}
