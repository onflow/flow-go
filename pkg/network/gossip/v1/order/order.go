package order

import (
	"context"
	"fmt"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

// Order is the evolved version of the deprecated tracker type of Gossip package.
// order offers a struct to internally track running gossip messages and their return values
type Order struct {
	Msg  *shared.GossipMessage
	Ctx  context.Context
	sync bool
	err  error
	resp []byte
	done chan struct{}
}

// NewOrder returns a new order instance
func NewOrder(ctx context.Context, msg *shared.GossipMessage, isSync bool) *Order {
	var d chan struct{}
	if isSync {
		// the done channel is made with a buffer of size 1 in case it was done but
		// no one was waiting for a result
		// TODO: Add a way to auto destruct orders which finished but are waiting
		// after x amount of time OR make buffer size 0 and enforce the consequences
		d = make(chan struct{}, 1)
	}
	return &Order{
		Msg:  msg,
		Ctx:  context.Background(),
		sync: isSync,
		done: d,
	}
}

// Done returns a channel. The channel is used to track the return values of a gossip message.
func (o *Order) Done() <-chan struct{} {
	if o.isSync() {
		return o.done
	}
	return make(chan struct{})
}

// Result returns the result of execution of the gossip message
func (o *Order) Result() ([]byte, error) {
	return o.resp, o.err
}

// Fill updates the order with the result of the execution of the gossip message,
// it also signals the done channel notifying that the execution is done
func (o *Order) Fill(resp []byte, err error) {
	if !o.isSync() {
		//an async message should not being tracked and hence does not have a result
		return
	}
	//updating results of the order ticket
	o.resp = resp
	o.err = err
	//notifying done channel
	o.done <- struct{}{}
}

// isSync checks if this Order is for a sync or async response
func (o *Order) isSync() bool {
	return o.sync
}

func (o *Order) String() string {
	return fmt.Sprintf("Order message: [%v], sync: %v", o.Msg, o.sync)
}

// Valid checks if the order is valid. Currently valid is defined as having a non-nil message field in the order ticket
func Valid(o *Order) bool {
	return o != nil && o.Msg != nil
}
