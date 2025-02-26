// Code generated by mockery v2.43.2. DO NOT EDIT.

package mempool

import (
	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// MutableBackData is an autogenerated mock type for the MutableBackData type
type MutableBackData struct {
	mock.Mock
}

// Add provides a mock function with given fields: entityID, entity
func (_m *MutableBackData) Add(entityID flow.Identifier, entity flow.Entity) bool {
	ret := _m.Called(entityID, entity)

	if len(ret) == 0 {
		panic("no return value specified for Add")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier, flow.Entity) bool); ok {
		r0 = rf(entityID, entity)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Adjust provides a mock function with given fields: entityID, f
func (_m *MutableBackData) Adjust(entityID flow.Identifier, f func(flow.Entity) flow.Entity) (flow.Entity, bool) {
	ret := _m.Called(entityID, f)

	if len(ret) == 0 {
		panic("no return value specified for Adjust")
	}

	var r0 flow.Entity
	var r1 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier, func(flow.Entity) flow.Entity) (flow.Entity, bool)); ok {
		return rf(entityID, f)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, func(flow.Entity) flow.Entity) flow.Entity); ok {
		r0 = rf(entityID, f)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Entity)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, func(flow.Entity) flow.Entity) bool); ok {
		r1 = rf(entityID, f)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// AdjustWithInit provides a mock function with given fields: entityID, adjust, init
func (_m *MutableBackData) AdjustWithInit(entityID flow.Identifier, adjust func(flow.Entity) flow.Entity, init func() flow.Entity) (flow.Entity, bool) {
	ret := _m.Called(entityID, adjust, init)

	if len(ret) == 0 {
		panic("no return value specified for AdjustWithInit")
	}

	var r0 flow.Entity
	var r1 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier, func(flow.Entity) flow.Entity, func() flow.Entity) (flow.Entity, bool)); ok {
		return rf(entityID, adjust, init)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, func(flow.Entity) flow.Entity, func() flow.Entity) flow.Entity); ok {
		r0 = rf(entityID, adjust, init)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Entity)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, func(flow.Entity) flow.Entity, func() flow.Entity) bool); ok {
		r1 = rf(entityID, adjust, init)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// All provides a mock function with given fields:
func (_m *MutableBackData) All() map[flow.Identifier]flow.Entity {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for All")
	}

	var r0 map[flow.Identifier]flow.Entity
	if rf, ok := ret.Get(0).(func() map[flow.Identifier]flow.Entity); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[flow.Identifier]flow.Entity)
		}
	}

	return r0
}

// ByID provides a mock function with given fields: entityID
func (_m *MutableBackData) ByID(entityID flow.Identifier) (flow.Entity, bool) {
	ret := _m.Called(entityID)

	if len(ret) == 0 {
		panic("no return value specified for ByID")
	}

	var r0 flow.Entity
	var r1 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) (flow.Entity, bool)); ok {
		return rf(entityID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) flow.Entity); ok {
		r0 = rf(entityID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Entity)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(entityID)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Clear provides a mock function with given fields:
func (_m *MutableBackData) Clear() {
	_m.Called()
}

// Entities provides a mock function with given fields:
func (_m *MutableBackData) Entities() []flow.Entity {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Entities")
	}

	var r0 []flow.Entity
	if rf, ok := ret.Get(0).(func() []flow.Entity); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.Entity)
		}
	}

	return r0
}

// GetWithInit provides a mock function with given fields: entityID, init
func (_m *MutableBackData) GetWithInit(entityID flow.Identifier, init func() flow.Entity) (flow.Entity, bool) {
	ret := _m.Called(entityID, init)

	if len(ret) == 0 {
		panic("no return value specified for GetWithInit")
	}

	var r0 flow.Entity
	var r1 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier, func() flow.Entity) (flow.Entity, bool)); ok {
		return rf(entityID, init)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier, func() flow.Entity) flow.Entity); ok {
		r0 = rf(entityID, init)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Entity)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier, func() flow.Entity) bool); ok {
		r1 = rf(entityID, init)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Has provides a mock function with given fields: entityID
func (_m *MutableBackData) Has(entityID flow.Identifier) bool {
	ret := _m.Called(entityID)

	if len(ret) == 0 {
		panic("no return value specified for Has")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) bool); ok {
		r0 = rf(entityID)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// Identifiers provides a mock function with given fields:
func (_m *MutableBackData) Identifiers() flow.IdentifierList {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Identifiers")
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

// Remove provides a mock function with given fields: entityID
func (_m *MutableBackData) Remove(entityID flow.Identifier) (flow.Entity, bool) {
	ret := _m.Called(entityID)

	if len(ret) == 0 {
		panic("no return value specified for Remove")
	}

	var r0 flow.Entity
	var r1 bool
	if rf, ok := ret.Get(0).(func(flow.Identifier) (flow.Entity, bool)); ok {
		return rf(entityID)
	}
	if rf, ok := ret.Get(0).(func(flow.Identifier) flow.Entity); ok {
		r0 = rf(entityID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(flow.Entity)
		}
	}

	if rf, ok := ret.Get(1).(func(flow.Identifier) bool); ok {
		r1 = rf(entityID)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// Size provides a mock function with given fields:
func (_m *MutableBackData) Size() uint {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Size")
	}

	var r0 uint
	if rf, ok := ret.Get(0).(func() uint); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint)
	}

	return r0
}

// NewMutableBackData creates a new instance of MutableBackData. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMutableBackData(t interface {
	mock.TestingT
	Cleanup(func())
}) *MutableBackData {
	mock := &MutableBackData{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
