// Code generated by mockery v1.0.0. DO NOT EDIT.

package mocknetwork

import (
	flow "github.com/onflow/flow-go/model/flow"
	mock "github.com/stretchr/testify/mock"
)

// MessageProcessor is an autogenerated mock type for the MessageProcessor type
type MessageProcessor struct {
	mock.Mock
}

// Process provides a mock function with given fields: originID, message
func (_m *MessageProcessor) Process(originID flow.Identifier, message interface{}) error {
	ret := _m.Called(originID, message)

	var r0 error
	if rf, ok := ret.Get(0).(func(flow.Identifier, interface{}) error); ok {
		r0 = rf(originID, message)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
