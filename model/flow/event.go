// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"fmt"
	"github.com/dapperlabs/flow-go/crypto"
	"strings"
)

type Event struct {
	// TxHash is the hash of the transaction this event was emitted from.
	TxHash crypto.Hash
	// Type is the qualified event type.
	Type string
	// Values is a map of all the parameters to the event, keys are parameter
	// names, values are the parameter values and must be primitive types.
	Values map[string]interface{}
	// Index defines the ordering of events in a transaction. The first event
	// emitted has index 0, the second has index 1, and so on.
	Index int
}

// String returns the string representation of this event.
func (e Event) String() string {
	var values strings.Builder

	i := 0
	for key, value := range e.Values {
		if i > 0 {
			values.WriteString(", ")
		}

		values.WriteString(fmt.Sprintf("%s: %s", key, value))
		i++
	}

	return fmt.Sprintf("%s(%s)", e.Type, values.String())
}

// ID returns a canonical identifier that is guaranteed to be unique.
func (e Event) ID() string {
	b, _ := e.Encode()
	_ = b
	// TODO hash here
	return ""
}

func (e Event) Encode() ([]byte, error) {
	w := wrapEvent(e)
	_ = w
	// TODO encode here
	return nil, nil
}

// Defines only the fields needed to uniquely identify an event.
type eventWrapper struct {
	TxHash []byte
	Index  int
}

func wrapEvent(e Event) eventWrapper {
	return eventWrapper{
		TxHash: e.TxHash,
		Index:  e.Index,
	}
}
