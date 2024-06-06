// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocknetwork

import (
	flow "github.com/onflow/flow-go/model/flow"
	channels "github.com/onflow/flow-go/network/channels"

	message "github.com/onflow/flow-go/network/message"

	mock "github.com/stretchr/testify/mock"
)

// IncomingMessageScope is an autogenerated mock type for the IncomingMessageScope type
type IncomingMessageScope struct {
	mock.Mock
}

// Channel provides a mock function with given fields:
func (_m *IncomingMessageScope) Channel() channels.Channel {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Channel")
	}

	var r0 channels.Channel
	if rf, ok := ret.Get(0).(func() channels.Channel); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(channels.Channel)
	}

	return r0
}

// DecodedPayload provides a mock function with given fields:
func (_m *IncomingMessageScope) DecodedPayload() interface{} {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for DecodedPayload")
	}

	var r0 interface{}
	if rf, ok := ret.Get(0).(func() interface{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	return r0
}

// EventID provides a mock function with given fields:
func (_m *IncomingMessageScope) EventID() []byte {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for EventID")
	}

	var r0 []byte
	if rf, ok := ret.Get(0).(func() []byte); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	return r0
}

// OriginId provides a mock function with given fields:
func (_m *IncomingMessageScope) OriginId() flow.Identifier {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for OriginId")
	}

	var r0 flow.Identifier
	if rf, ok := ret.Get(0).(func() flow.Identifier); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Identifier)
		}
	}

	return r0
}

// PayloadType provides a mock function with given fields:
func (_m *IncomingMessageScope) PayloadType() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for PayloadType")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Proto provides a mock function with given fields:
func (_m *IncomingMessageScope) Proto() *message.Message {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Proto")
	}

	var r0 *message.Message
	if rf, ok := ret.Get(0).(func() *message.Message); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*message.Message)
		}
	}

	return r0
}

// Protocol provides a mock function with given fields:
func (_m *IncomingMessageScope) Protocol() message.ProtocolType {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Protocol")
	}

	var r0 message.ProtocolType
	if rf, ok := ret.Get(0).(func() message.ProtocolType); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(message.ProtocolType)
	}

	return r0
}

// Size provides a mock function with given fields:
func (_m *IncomingMessageScope) Size() int {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Size")
	}

	var r0 int
	if rf, ok := ret.Get(0).(func() int); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// TargetIDs provides a mock function with given fields:
func (_m *IncomingMessageScope) TargetIDs() flow.IdentifierList {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for TargetIDs")
	}

	var r0 flow.IdentifierList
	if rf, ok := ret.Get(0).(func() flow.IdentifierList); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.IdentifierList)
		}
	}

	return r0
}

// NewIncomingMessageScope creates a new instance of IncomingMessageScope. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewIncomingMessageScope(t interface {
	mock.TestingT
	Cleanup(func())
}) *IncomingMessageScope {
	mock := &IncomingMessageScope{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
