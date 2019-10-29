package order

import (
	"context"
	"fmt"
	"time"

	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

func ExampleNewSync() {
	msg, _ := generateMessage([]byte("hello"), []string{"example.org:1234"}, 1)
	fmt.Println(NewSync(context.Background(), msg))
	// Output: Order message: [ Payload:"hello" messageType:1 Recipients:"example.org:1234"  ], sync: true
}

func ExampleNewAsync() {
	msg, _ := generateMessage([]byte("hello"), []string{"example.org:1234"}, 1)
	fmt.Println(NewAsync(context.Background(), msg))
	// Output: Order message: [ Payload:"hello" messageType:1 Recipients:"example.org:1234"  ], sync: false
}

func ExampleOrder() {
	// temporary gossip message for example
	msg, _ := generateMessage([]byte("hello"), []string{"example.org:1234"}, 1)

	// creating a Sync Order
	ord := NewSync(context.Background(), msg)

	// fillig the order's results
	// (will also notify callers who wait for ord to get finished)
	go func() {
		ord.Fill([]byte("Response"), nil)
	}()

	t := time.After(1 * time.Second)
	select {
	case <-t:
	case <-ord.Done():
		resp, _ := ord.Result()
		fmt.Println(string(resp))
	}
	// Output: Response

}

// generateGossipMessage initializes a new gossip message made from the given inputs
//Note: similar function already declared in message.go in gossip package. We redeclare it here in a simpler form for
//test essentially since go does not allow cycle of imports in tests
func generateMessage(payloadBytes []byte, recipients []string, msgType uint64) (*messages.GossipMessage, error) {
	return &messages.GossipMessage{
		Payload:     payloadBytes,
		MessageType: msgType,
		Recipients:  recipients,
	}, nil
}
