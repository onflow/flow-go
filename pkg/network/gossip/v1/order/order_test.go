package order

import (
	"context"
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

// generating and printing an order for a gossip message in synchronous mode
func ExampleNewOrder() {
	msg, _ := generateMessage([]byte("hello"), []string{"example.org:1234"}, "display")
	fmt.Println(NewOrder(context.Background(), msg, true))
	// Output: Order message: [Payload:"hello" messageType:"display" Recipients:"example.org:1234" ], sync: true
}

// generating an order for a message in synchronous mode, filling its result, and testing its done status
func ExampleOrder() {
	// temporary gossip message for example
	msg, _ := generateMessage([]byte("hello"), []string{"example.org:1234"}, "display")

	// creating a Sync Order
	ord := NewOrder(context.Background(), msg, true)

	// filling the order's results
	// (will also notify callers who wait for ord to get finished)
	go func() {
		ord.Fill([]byte("Response"), nil)
	}()

	t := time.After(1 * time.Second)
	select {
	case <-t:
	case <-ord.Done():
		resp, err := ord.Result()
		fmt.Println(string(resp))
		fmt.Println(err)
	}
	// Output:
	// Response
	// <nil>

}

// generateGossipMessage initializes a new gossip message made from the given inputs
//Note: similar function already declared in message.go in gnode package. We redeclare it here in a simpler form for
//test essentially since go does not allow cycle of imports in tests
func generateMessage(payloadBytes []byte, recipients []string, msgType string) (*shared.GossipMessage, error) {
	return &shared.GossipMessage{
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}, nil
}
