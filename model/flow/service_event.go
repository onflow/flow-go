package flow

import (
	"encoding/json"
	"fmt"

	"github.com/vmihailenco/msgpack/v4"
)

const (
	ServiceEventSetup  = "setup"
	ServiceEventCommit = "commit"
)

// ServiceEvent represents a service event, which is a special event that when
// emitted from a service account smart contract, is propagated to the protocol
// and included in blocks. Service events typically cause changes to the
// protocol state. See EpochSetup and EpochCommit events in this package for examples.
//
// This type represents a generic service event and primarily exists to simplify
// encoding and decoding.
type ServiceEvent struct {
	Type  string
	Event interface{}
}

func (se *ServiceEvent) UnmarshalJSON(b []byte) error {

	var enc map[string]interface{}
	err := json.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	tp, ok := enc["Type"].(string)
	if !ok {
		return fmt.Errorf("missing type key")
	}
	ev, ok := enc["Event"]
	if !ok {
		return fmt.Errorf("missing event key")
	}

	// re-marshal the event, we'll unmarshal it into the appropriate type
	evb, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	var event interface{}
	switch tp {
	case ServiceEventSetup:
		setup := new(EpochSetup)
		err = json.Unmarshal(evb, setup)
		if err != nil {
			return err
		}
		event = setup
	case ServiceEventCommit:
		commit := new(EpochCommit)
		err = json.Unmarshal(evb, commit)
		if err != nil {
			return err
		}
		event = commit
	default:
		return fmt.Errorf("invalid type: %s", tp)
	}

	*se = ServiceEvent{
		Type:  tp,
		Event: event,
	}
	return nil
}

func (se *ServiceEvent) UnmarshalMsgpack(b []byte) error {

	var enc map[string]interface{}
	err := msgpack.Unmarshal(b, &enc)
	if err != nil {
		return err
	}

	tp, ok := enc["Type"].(string)
	if !ok {
		return fmt.Errorf("missing type key")
	}
	ev, ok := enc["Event"]
	if !ok {
		return fmt.Errorf("missing event key")
	}

	// re-marshal the event, we'll unmarshal it into the appropriate type
	evb, err := msgpack.Marshal(ev)
	if err != nil {
		return err
	}

	var event interface{}
	switch tp {
	case ServiceEventSetup:
		setup := new(EpochSetup)
		err = msgpack.Unmarshal(evb, setup)
		if err != nil {
			return err
		}
		event = setup
	case ServiceEventCommit:
		commit := new(EpochCommit)
		err = msgpack.Unmarshal(evb, commit)
		if err != nil {
			return err
		}
		event = commit
	default:
		return fmt.Errorf("invalid type: %s", tp)
	}

	*se = ServiceEvent{
		Type:  tp,
		Event: event,
	}
	return nil
}
