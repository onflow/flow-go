package subscription

import (
	"fmt"
)

// HandleSubscription is a generic handler for subscriptions to a specific type. It continuously listens to the subscription channel,
// handles the received responses, and sends the processed information to the client via the provided stream using handleResponse.
//
// Parameters:
// - sub: The subscription.
// - handleResponse: The function responsible for handling the response of the subscribed type.
//
// No errors are expected during normal operations.
func HandleSubscription[T any](sub Subscription, handleResponse func(resp T) error) error {
	for {
		v, ok := <-sub.Channel()
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
	}
}

// HandleResponse processes a generic response of type and sends it to the provided channel.
//
// Parameters:
// - send: The channel to which the processed response is sent.
// - transform: A function to transform the response into the expected interface{} type.
//
// No errors are expected during normal operations.
func HandleResponse[T any](send chan<- any, transform func(resp T) (any, error)) func(resp T) error {
	return func(response T) error {
		// Transform the response
		resp, err := transform(response)
		if err != nil {
			return fmt.Errorf("failed to transform response: %w", err)
		}

		// send to the channel
		send <- resp

		return nil
	}
}
