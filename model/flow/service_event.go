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
	ServiceEventSetup                       ServiceEventType = "setup"
	ServiceEventCommit                      ServiceEventType = "commit"
	ServiceEventVersionBeacon               ServiceEventType = "version-beacon"
	ServiceEventProtocolStateVersionUpgrade ServiceEventType = "protocol-state-version-upgrade"
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

// serviceEventUnmarshalWrapper is a version of ServiceEvent used to unmarshal
// into a wrapper with a specific event type.
type serviceEventUnmarshalWrapper[E any] struct {
	Type  ServiceEventType
	Event E
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

// ServiceEventMarshaller marshals and unmarshals all types of service events.
type ServiceEventMarshaller interface {
	// UnmarshalWrapped unmarshals the service event and returns it as a wrapped ServiceEvent type.
	// The input bytes must be encoded as a generic wrapped ServiceEvent type.
	UnmarshalWrapped(b []byte) (ServiceEvent, error)
	// UnmarshalWithType unmarshals the service event and returns it as a wrapped ServiceEvent type.
	// The input bytes must be encoded as a specific event type (for example, EpochSetup).
	UnmarshalWithType(b []byte, eventType ServiceEventType) (ServiceEvent, error)
}

type marshallerImpl struct {
	marshalFunc   func(v interface{}) ([]byte, error)
	unmarshalFunc func(data []byte, v interface{}) error
}

var _ ServiceEventMarshaller = (*marshallerImpl)(nil)

var (
	ServiceEventJSONMarshaller = marshallerImpl{
		marshalFunc:   json.Marshal,
		unmarshalFunc: json.Unmarshal,
	}
	ServiceEventMSGPACKMarshaller = marshallerImpl{
		marshalFunc:   msgpack.Marshal,
		unmarshalFunc: msgpack.Unmarshal,
	}
	ServiceEventCBORMarshaller = marshallerImpl{
		marshalFunc:   cborcodec.EncMode.Marshal,
		unmarshalFunc: cbor.Unmarshal,
	}
)

// UnmarshalWrapped unmarshals the service event and returns it as a wrapped ServiceEvent type.
// The input bytes must be encoded as a generic wrapped ServiceEvent type.
func (marshaller marshallerImpl) UnmarshalWrapped(b []byte) (ServiceEvent, error) {
	var eventTypeWrapper struct {
		Type ServiceEventType
	}
	err := marshaller.unmarshalFunc(b, &eventTypeWrapper)
	if err != nil {
		return ServiceEvent{}, err
	}
	eventType := eventTypeWrapper.Type

	var event any
	switch eventType {
	case ServiceEventSetup:
		event, err = unmarshalWrapped[EpochSetup](b, marshaller)
	case ServiceEventCommit:
		event, err = unmarshalWrapped[EpochCommit](b, marshaller)
	case ServiceEventVersionBeacon:
		event, err = unmarshalWrapped[VersionBeacon](b, marshaller)
	case ServiceEventProtocolStateVersionUpgrade:
		event, err = unmarshalWrapped[ProtocolStateVersionUpgrade](b, marshaller)
	default:
		return ServiceEvent{}, fmt.Errorf("invalid type: %s", eventType)
	}

	if err != nil {
		return ServiceEvent{}, fmt.Errorf("failed to unmarshal to service event to type %s: %w", eventType, err)
	}
	return ServiceEvent{
		Type:  eventType,
		Event: event,
	}, nil
}

// unmarshalWrapped is a helper function for UnmarshalWrapped which unmarshals the
// Event portion of a ServiceEvent into a specific typed structure.
// No errors are expected during normal operation.
func unmarshalWrapped[E any](b []byte, marshaller marshallerImpl) (*E, error) {
	eventWrapper := serviceEventUnmarshalWrapper[E]{}
	err := marshaller.unmarshalFunc(b, &eventWrapper)
	if err != nil {
		return nil, err
	}

	return &eventWrapper.Event, nil
}

// UnmarshalWithType unmarshals the service event and returns it as a wrapped ServiceEvent type.
// The input bytes must be encoded as a specific event type (for example, EpochSetup).
func (marshaller marshallerImpl) UnmarshalWithType(b []byte, eventType ServiceEventType) (ServiceEvent, error) {
	var event interface{}
	switch eventType {
	case ServiceEventSetup:
		event = new(EpochSetup)
	case ServiceEventCommit:
		event = new(EpochCommit)
	case ServiceEventVersionBeacon:
		event = new(VersionBeacon)
	case ServiceEventProtocolStateVersionUpgrade:
		event = new(ProtocolStateVersionUpgrade)
	default:
		return ServiceEvent{}, fmt.Errorf("invalid type: %s", eventType)
	}

	err := marshaller.unmarshalFunc(b, event)
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
	e, err := ServiceEventJSONMarshaller.UnmarshalWrapped(b)
	if err != nil {
		return err
	}
	*se = e
	return nil
}

func (se *ServiceEvent) UnmarshalMsgpack(b []byte) error {
	e, err := ServiceEventMSGPACKMarshaller.UnmarshalWrapped(b)
	if err != nil {
		return err
	}
	*se = e
	return nil
}

func (se *ServiceEvent) UnmarshalCBOR(b []byte) error {
	e, err := ServiceEventCBORMarshaller.UnmarshalWrapped(b)
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
	case ServiceEventProtocolStateVersionUpgrade:
		version, ok := se.Event.(*ProtocolStateVersionUpgrade)
		if !ok {
			return false, fmt.Errorf(
				"internal invalid type for ProtocolStateVersionUpgrade: %T",
				se.Event,
			)
		}
		otherVersion, ok := other.Event.(*ProtocolStateVersionUpgrade)
		if !ok {
			return false,
				fmt.Errorf(
					"internal invalid type for ProtocolStateVersionUpgrade: %T",
					other.Event,
				)
		}
		return version.EqualTo(otherVersion), nil

	default:
		return false, fmt.Errorf("unknown serice event type: %s", se.Type)
	}
}
