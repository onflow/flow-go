package flow

import (
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v4"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
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

// ServiceEventList is a handy container to enable comparisons
type ServiceEventList []ServiceEvent

func (sel ServiceEventList) EqualTo(other ServiceEventList) (bool, error) {
	if len(sel) != len(other) {
		return false, nil
	}

	for i, se := range sel {
		equalTo, err := se.EqualTo(&other[i])
		if err != nil {
			return false, fmt.Errorf("error while comparing service event index %d: %w", i, err)
		}
		if !equalTo {
			return false, nil
		}
	}

	return true, nil
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

func (se *ServiceEvent) UnmarshalCBOR(b []byte) error {

	var enc map[string]interface{}
	err := cbor.Unmarshal(b, &enc)
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

	evb, err := cborcodec.EncMode.Marshal(ev)
	if err != nil {
		return err
	}

	var event interface{}
	switch tp {
	case ServiceEventSetup:
		setup := new(EpochSetup)
		err = cbor.Unmarshal(evb, setup)
		if err != nil {
			return err
		}
		event = setup
	case ServiceEventCommit:
		commit := new(EpochCommit)
		err = cbor.Unmarshal(evb, commit)
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

func (se *ServiceEvent) EqualTo(other *ServiceEvent) (bool, error) {
	if se.Type != other.Type {
		return false, nil
	}
	switch se.Type {
	case ServiceEventSetup:
		setup, ok := se.Event.(*EpochSetup)
		if !ok {
			return false, fmt.Errorf("internal invalid type for ServiceEventSetup: %T", se.Event)
		}
		otherSetup, ok := other.Event.(*EpochSetup)
		if !ok {
			return false, fmt.Errorf("internal invalid type for ServiceEventSetup: %T", other.Event)
		}
		return setup.EqualTo(otherSetup), nil

	case ServiceEventCommit:
		commit, ok := se.Event.(*EpochCommit)
		if !ok {
			return false, fmt.Errorf("internal invalid type for ServiceEventCommit: %T", se.Event)
		}
		otherCommit, ok := other.Event.(*EpochCommit)
		if !ok {
			return false, fmt.Errorf("internal invalid type for ServiceEventCommit: %T", other.Event)
		}
		return commit.EqualTo(otherCommit), nil
	default:
		return false, fmt.Errorf("unknown serice event type: %s", se.Type)
	}
}
