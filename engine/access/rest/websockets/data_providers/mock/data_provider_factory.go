// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	context "context"

	data_providers "github.com/onflow/flow-go/engine/access/rest/websockets/data_providers"
	mock "github.com/stretchr/testify/mock"

	models "github.com/onflow/flow-go/engine/access/rest/websockets/models"
)

// DataProviderFactory is an autogenerated mock type for the DataProviderFactory type
type DataProviderFactory struct {
	mock.Mock
}

// NewDataProvider provides a mock function with given fields: ctx, subID, topic, args, stream
func (_m *DataProviderFactory) NewDataProvider(ctx context.Context, subID string, topic string, args models.Arguments, stream chan<- interface{}) (data_providers.DataProvider, error) {
	ret := _m.Called(ctx, subID, topic, args, stream)

	if len(ret) == 0 {
		panic("no return value specified for NewDataProvider")
	}

	var r0 data_providers.DataProvider
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, string, models.Arguments, chan<- interface{}) (data_providers.DataProvider, error)); ok {
		return rf(ctx, subID, topic, args, stream)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, string, models.Arguments, chan<- interface{}) data_providers.DataProvider); ok {
		r0 = rf(ctx, subID, topic, args, stream)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(data_providers.DataProvider)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, string, models.Arguments, chan<- interface{}) error); ok {
		r1 = rf(ctx, subID, topic, args, stream)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewDataProviderFactory creates a new instance of DataProviderFactory. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewDataProviderFactory(t interface {
	mock.TestingT
	Cleanup(func())
}) *DataProviderFactory {
	mock := &DataProviderFactory{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
