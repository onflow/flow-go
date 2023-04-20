package flow

import (
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
	"github.com/vmihailenco/msgpack/v4"

	cborcodec "github.com/onflow/flow-go/model/encoding/cbor"
)

type ServiceEventType string

// String returns the string representation of the service event type.
// TODO: this should not be needed. We should use ServiceEventType directly everywhere.
func (set ServiceEventType) String() string {
	return string(set)
}

const (
	ServiceEventSetup         ServiceEventType = "setup"
	ServiceEventCommit        ServiceEventType = "commit"
	ServiceEventVersionBeacon ServiceEventType = "version-beacon"
)

// ServiceEvent represents a service event, which is a special event that when
// emitted from a service account smart contract, is propagated to the protocol
// and included in blocks. Service events typically cause changes to the
// protocol state. See EpochSetup and EpochCommit events in this package for examples.
//
// This type represents a generic service event and primarily exists to simplify
// encoding and decoding.
type ServiceEvent struct {
	Type  ServiceEventType
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
			return false, fmt.Errorf(
				"error while comparing service event index %d: %w",
				i,
				err,
			)
		}
		if !equalTo {
			return false, nil
		}
	}

	return true, nil
}

type ServiceEventMarshaller interface {
	Unmarshal(b []byte) (ServiceEvent, error)
	UnmarshalWithType(
		b []byte,
		eventType ServiceEventType,
	) (
		ServiceEvent,
		error,
	)
}

type marshallerImpl struct {
	MarshalFunc   func(v interface{}) ([]byte, error)
	UnmarshalFunc func(data []byte, v interface{}) error
}

var (
	ServiceEventJSONMarshaller = marshallerImpl{
		MarshalFunc:   json.Marshal,
		UnmarshalFunc: json.Unmarshal,
	}
	ServiceEventMSGPACKMarshaller = marshallerImpl{
		MarshalFunc:   msgpack.Marshal,
		UnmarshalFunc: msgpack.Unmarshal,
	}
	ServiceEventCBORMarshaller = marshallerImpl{
		MarshalFunc:   cborcodec.EncMode.Marshal,
		UnmarshalFunc: cbor.Unmarshal,
	}
)

func (marshaller marshallerImpl) Unmarshal(b []byte) (
	ServiceEvent,
	error,
) {
	var enc map[string]interface{}
	err := marshaller.UnmarshalFunc(b, &enc)
	if err != nil {
		return ServiceEvent{}, err
	}

	tp, ok := enc["Type"].(string)
	if !ok {
		return ServiceEvent{}, fmt.Errorf("missing type key")
	}
	ev, ok := enc["Event"]
	if !ok {
		return ServiceEvent{}, fmt.Errorf("missing event key")
	}

	// re-marshal the event, we'll unmarshal it into the appropriate type
	evb, err := marshaller.MarshalFunc(ev)
	if err != nil {
		return ServiceEvent{}, err
	}

	return marshaller.UnmarshalWithType(evb, ServiceEventType(tp))
}

func (marshaller marshallerImpl) UnmarshalWithType(
	b []byte,
	eventType ServiceEventType,
) (ServiceEvent, error) {
	var event interface{}
	switch eventType {
	case ServiceEventSetup:
		event = new(EpochSetup)
	case ServiceEventCommit:
		event = new(EpochCommit)
	case ServiceEventVersionBeacon:
		event = new(VersionBeacon)
	default:
		return ServiceEvent{}, fmt.Errorf("invalid type: %s", eventType)
	}

	err := marshaller.UnmarshalFunc(b, event)
	if err != nil {
		return ServiceEvent{},
			fmt.Errorf(
				"failed to unmarshal to service event ot type %s: %w",
				eventType,
				err,
			)
	}

	return ServiceEvent{
		Type:  eventType,
		Event: event,
	}, nil
}

func (se *ServiceEvent) UnmarshalJSON(b []byte) error {
	e, err := ServiceEventJSONMarshaller.Unmarshal(b)
	if err != nil {
		return err
	}
	*se = e
	return nil
}

func (se *ServiceEvent) UnmarshalMsgpack(b []byte) error {
	e, err := ServiceEventMSGPACKMarshaller.Unmarshal(b)
	if err != nil {
		return err
	}
	*se = e
	return nil
}

func (se *ServiceEvent) UnmarshalCBOR(b []byte) error {
	e, err := ServiceEventCBORMarshaller.Unmarshal(b)
	if err != nil {
		return err
	}
	*se = e
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
			return false, fmt.Errorf(
				"internal invalid type for ServiceEventSetup: %T",
				se.Event,
			)
		}
		otherSetup, ok := other.Event.(*EpochSetup)
		if !ok {
			return false, fmt.Errorf(
				"internal invalid type for ServiceEventSetup: %T",
				other.Event,
			)
		}
		return setup.EqualTo(otherSetup), nil

	case ServiceEventCommit:
		commit, ok := se.Event.(*EpochCommit)
		if !ok {
			return false, fmt.Errorf(
				"internal invalid type for ServiceEventCommit: %T",
				se.Event,
			)
		}
		otherCommit, ok := other.Event.(*EpochCommit)
		if !ok {
			return false, fmt.Errorf(
				"internal invalid type for ServiceEventCommit: %T",
				other.Event,
			)
		}
		return commit.EqualTo(otherCommit), nil

	case ServiceEventVersionBeacon:
		version, ok := se.Event.(*VersionBeacon)
		if !ok {
			return false, fmt.Errorf(
				"internal invalid type for ServiceEventVersionBeacon: %T",
				se.Event,
			)
		}
		otherVersion, ok := other.Event.(*VersionBeacon)
		if !ok {
			return false,
				fmt.Errorf(
					"internal invalid type for ServiceEventVersionBeacon: %T",
					other.Event,
				)
		}
		return version.EqualTo(otherVersion), nil

	default:
		return false, fmt.Errorf("unknown serice event type: %s", se.Type)
	}
}
