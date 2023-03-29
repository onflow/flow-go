// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	ledger "github.com/onflow/flow-go/ledger"
	mock "github.com/stretchr/testify/mock"
)

// Reporter is an autogenerated mock type for the Reporter type
type Reporter struct {
	mock.Mock
}

// Name provides a mock function with given fields:
func (_m *Reporter) Name() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// Report provides a mock function with given fields: payloads, statecommitment
func (_m *Reporter) Report(payloads []ledger.Payload, statecommitment ledger.State) error {
	ret := _m.Called(payloads, statecommitment)

	var r0 error
	if rf, ok := ret.Get(0).(func([]ledger.Payload, ledger.State) error); ok {
		r0 = rf(payloads, statecommitment)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewReporter interface {
	mock.TestingT
	Cleanup(func())
}

// NewReporter creates a new instance of Reporter. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewReporter(t mockConstructorTestingTNewReporter) *Reporter {
	mock := &Reporter{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
