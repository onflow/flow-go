// Code generated by mockery v2.43.2. DO NOT EDIT.

package mock

import (
	context "context"

	access "github.com/onflow/flow-go/access"

	entities "github.com/onflow/flow/protobuf/go/flow/entities"

	flow "github.com/onflow/flow-go/model/flow"

	mock "github.com/stretchr/testify/mock"
)

// API is an autogenerated mock type for the API type
type API struct {
	mock.Mock
}

// ExecuteScriptAtBlockHeight provides a mock function with given fields: ctx, blockHeight, script, arguments
func (_m *API) ExecuteScriptAtBlockHeight(ctx context.Context, blockHeight uint64, script []byte, arguments [][]byte) ([]byte, error) {
	ret := _m.Called(ctx, blockHeight, script, arguments)

	if len(ret) == 0 {
		panic("no return value specified for ExecuteScriptAtBlockHeight")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64, []byte, [][]byte) ([]byte, error)); ok {
		return rf(ctx, blockHeight, script, arguments)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint64, []byte, [][]byte) []byte); ok {
		r0 = rf(ctx, blockHeight, script, arguments)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint64, []byte, [][]byte) error); ok {
		r1 = rf(ctx, blockHeight, script, arguments)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteScriptAtBlockID provides a mock function with given fields: ctx, blockID, script, arguments
func (_m *API) ExecuteScriptAtBlockID(ctx context.Context, blockID flow.Identifier, script []byte, arguments [][]byte) ([]byte, error) {
	ret := _m.Called(ctx, blockID, script, arguments)

	if len(ret) == 0 {
		panic("no return value specified for ExecuteScriptAtBlockID")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, []byte, [][]byte) ([]byte, error)); ok {
		return rf(ctx, blockID, script, arguments)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, []byte, [][]byte) []byte); ok {
		r0 = rf(ctx, blockID, script, arguments)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier, []byte, [][]byte) error); ok {
		r1 = rf(ctx, blockID, script, arguments)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteScriptAtLatestBlock provides a mock function with given fields: ctx, script, arguments
func (_m *API) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, arguments [][]byte) ([]byte, error) {
	ret := _m.Called(ctx, script, arguments)

	if len(ret) == 0 {
		panic("no return value specified for ExecuteScriptAtLatestBlock")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, []byte, [][]byte) ([]byte, error)); ok {
		return rf(ctx, script, arguments)
	}
	if rf, ok := ret.Get(0).(func(context.Context, []byte, [][]byte) []byte); ok {
		r0 = rf(ctx, script, arguments)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, []byte, [][]byte) error); ok {
		r1 = rf(ctx, script, arguments)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccount provides a mock function with given fields: ctx, address
func (_m *API) GetAccount(ctx context.Context, address flow.Address) (*flow.Account, error) {
	ret := _m.Called(ctx, address)

	if len(ret) == 0 {
		panic("no return value specified for GetAccount")
	}

	var r0 *flow.Account
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address) (*flow.Account, error)); ok {
		return rf(ctx, address)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address) *flow.Account); ok {
		r0 = rf(ctx, address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Address) error); ok {
		r1 = rf(ctx, address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccountAtBlockHeight provides a mock function with given fields: ctx, address, height
func (_m *API) GetAccountAtBlockHeight(ctx context.Context, address flow.Address, height uint64) (*flow.Account, error) {
	ret := _m.Called(ctx, address, height)

	if len(ret) == 0 {
		panic("no return value specified for GetAccountAtBlockHeight")
	}

	var r0 *flow.Account
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address, uint64) (*flow.Account, error)); ok {
		return rf(ctx, address, height)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address, uint64) *flow.Account); ok {
		r0 = rf(ctx, address, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Address, uint64) error); ok {
		r1 = rf(ctx, address, height)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccountAtLatestBlock provides a mock function with given fields: ctx, address
func (_m *API) GetAccountAtLatestBlock(ctx context.Context, address flow.Address) (*flow.Account, error) {
	ret := _m.Called(ctx, address)

	if len(ret) == 0 {
		panic("no return value specified for GetAccountAtLatestBlock")
	}

	var r0 *flow.Account
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address) (*flow.Account, error)); ok {
		return rf(ctx, address)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Address) *flow.Account); ok {
		r0 = rf(ctx, address)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Account)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Address) error); ok {
		r1 = rf(ctx, address)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockByHeight provides a mock function with given fields: ctx, height
func (_m *API) GetBlockByHeight(ctx context.Context, height uint64) (*flow.Block, flow.BlockStatus, error) {
	ret := _m.Called(ctx, height)

	if len(ret) == 0 {
		panic("no return value specified for GetBlockByHeight")
	}

	var r0 *flow.Block
	var r1 flow.BlockStatus
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64) (*flow.Block, flow.BlockStatus, error)); ok {
		return rf(ctx, height)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint64) *flow.Block); ok {
		r0 = rf(ctx, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint64) flow.BlockStatus); ok {
		r1 = rf(ctx, height)
	} else {
		r1 = ret.Get(1).(flow.BlockStatus)
	}

	if rf, ok := ret.Get(2).(func(context.Context, uint64) error); ok {
		r2 = rf(ctx, height)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetBlockByID provides a mock function with given fields: ctx, id
func (_m *API) GetBlockByID(ctx context.Context, id flow.Identifier) (*flow.Block, flow.BlockStatus, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetBlockByID")
	}

	var r0 *flow.Block
	var r1 flow.BlockStatus
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.Block, flow.BlockStatus, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Block); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) flow.BlockStatus); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Get(1).(flow.BlockStatus)
	}

	if rf, ok := ret.Get(2).(func(context.Context, flow.Identifier) error); ok {
		r2 = rf(ctx, id)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetBlockHeaderByHeight provides a mock function with given fields: ctx, height
func (_m *API) GetBlockHeaderByHeight(ctx context.Context, height uint64) (*flow.Header, flow.BlockStatus, error) {
	ret := _m.Called(ctx, height)

	if len(ret) == 0 {
		panic("no return value specified for GetBlockHeaderByHeight")
	}

	var r0 *flow.Header
	var r1 flow.BlockStatus
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64) (*flow.Header, flow.BlockStatus, error)); ok {
		return rf(ctx, height)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint64) *flow.Header); ok {
		r0 = rf(ctx, height)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint64) flow.BlockStatus); ok {
		r1 = rf(ctx, height)
	} else {
		r1 = ret.Get(1).(flow.BlockStatus)
	}

	if rf, ok := ret.Get(2).(func(context.Context, uint64) error); ok {
		r2 = rf(ctx, height)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetBlockHeaderByID provides a mock function with given fields: ctx, id
func (_m *API) GetBlockHeaderByID(ctx context.Context, id flow.Identifier) (*flow.Header, flow.BlockStatus, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetBlockHeaderByID")
	}

	var r0 *flow.Header
	var r1 flow.BlockStatus
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.Header, flow.BlockStatus, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Header); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) flow.BlockStatus); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Get(1).(flow.BlockStatus)
	}

	if rf, ok := ret.Get(2).(func(context.Context, flow.Identifier) error); ok {
		r2 = rf(ctx, id)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetCollectionByID provides a mock function with given fields: ctx, id
func (_m *API) GetCollectionByID(ctx context.Context, id flow.Identifier) (*flow.LightCollection, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetCollectionByID")
	}

	var r0 *flow.LightCollection
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.LightCollection, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.LightCollection); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.LightCollection)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEventsForBlockIDs provides a mock function with given fields: ctx, eventType, blockIDs, requiredEventEncodingVersion
func (_m *API) GetEventsForBlockIDs(ctx context.Context, eventType string, blockIDs []flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error) {
	ret := _m.Called(ctx, eventType, blockIDs, requiredEventEncodingVersion)

	if len(ret) == 0 {
		panic("no return value specified for GetEventsForBlockIDs")
	}

	var r0 []flow.BlockEvents
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, []flow.Identifier, entities.EventEncodingVersion) ([]flow.BlockEvents, error)); ok {
		return rf(ctx, eventType, blockIDs, requiredEventEncodingVersion)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, []flow.Identifier, entities.EventEncodingVersion) []flow.BlockEvents); ok {
		r0 = rf(ctx, eventType, blockIDs, requiredEventEncodingVersion)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.BlockEvents)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, []flow.Identifier, entities.EventEncodingVersion) error); ok {
		r1 = rf(ctx, eventType, blockIDs, requiredEventEncodingVersion)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEventsForHeightRange provides a mock function with given fields: ctx, eventType, startHeight, endHeight, requiredEventEncodingVersion
func (_m *API) GetEventsForHeightRange(ctx context.Context, eventType string, startHeight uint64, endHeight uint64, requiredEventEncodingVersion entities.EventEncodingVersion) ([]flow.BlockEvents, error) {
	ret := _m.Called(ctx, eventType, startHeight, endHeight, requiredEventEncodingVersion)

	if len(ret) == 0 {
		panic("no return value specified for GetEventsForHeightRange")
	}

	var r0 []flow.BlockEvents
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string, uint64, uint64, entities.EventEncodingVersion) ([]flow.BlockEvents, error)); ok {
		return rf(ctx, eventType, startHeight, endHeight, requiredEventEncodingVersion)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string, uint64, uint64, entities.EventEncodingVersion) []flow.BlockEvents); ok {
		r0 = rf(ctx, eventType, startHeight, endHeight, requiredEventEncodingVersion)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]flow.BlockEvents)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string, uint64, uint64, entities.EventEncodingVersion) error); ok {
		r1 = rf(ctx, eventType, startHeight, endHeight, requiredEventEncodingVersion)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutionResultByID provides a mock function with given fields: ctx, id
func (_m *API) GetExecutionResultByID(ctx context.Context, id flow.Identifier) (*flow.ExecutionResult, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetExecutionResultByID")
	}

	var r0 *flow.ExecutionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.ExecutionResult, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.ExecutionResult); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutionResultForBlockID provides a mock function with given fields: ctx, blockID
func (_m *API) GetExecutionResultForBlockID(ctx context.Context, blockID flow.Identifier) (*flow.ExecutionResult, error) {
	ret := _m.Called(ctx, blockID)

	if len(ret) == 0 {
		panic("no return value specified for GetExecutionResultForBlockID")
	}

	var r0 *flow.ExecutionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.ExecutionResult, error)); ok {
		return rf(ctx, blockID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.ExecutionResult); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.ExecutionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetFullCollectionByID provides a mock function with given fields: ctx, id
func (_m *API) GetFullCollectionByID(ctx context.Context, id flow.Identifier) (*flow.Collection, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetFullCollectionByID")
	}

	var r0 *flow.Collection
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.Collection, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.Collection); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Collection)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestBlock provides a mock function with given fields: ctx, isSealed
func (_m *API) GetLatestBlock(ctx context.Context, isSealed bool) (*flow.Block, flow.BlockStatus, error) {
	ret := _m.Called(ctx, isSealed)

	if len(ret) == 0 {
		panic("no return value specified for GetLatestBlock")
	}

	var r0 *flow.Block
	var r1 flow.BlockStatus
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, bool) (*flow.Block, flow.BlockStatus, error)); ok {
		return rf(ctx, isSealed)
	}
	if rf, ok := ret.Get(0).(func(context.Context, bool) *flow.Block); ok {
		r0 = rf(ctx, isSealed)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Block)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, bool) flow.BlockStatus); ok {
		r1 = rf(ctx, isSealed)
	} else {
		r1 = ret.Get(1).(flow.BlockStatus)
	}

	if rf, ok := ret.Get(2).(func(context.Context, bool) error); ok {
		r2 = rf(ctx, isSealed)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetLatestBlockHeader provides a mock function with given fields: ctx, isSealed
func (_m *API) GetLatestBlockHeader(ctx context.Context, isSealed bool) (*flow.Header, flow.BlockStatus, error) {
	ret := _m.Called(ctx, isSealed)

	if len(ret) == 0 {
		panic("no return value specified for GetLatestBlockHeader")
	}

	var r0 *flow.Header
	var r1 flow.BlockStatus
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context, bool) (*flow.Header, flow.BlockStatus, error)); ok {
		return rf(ctx, isSealed)
	}
	if rf, ok := ret.Get(0).(func(context.Context, bool) *flow.Header); ok {
		r0 = rf(ctx, isSealed)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.Header)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, bool) flow.BlockStatus); ok {
		r1 = rf(ctx, isSealed)
	} else {
		r1 = ret.Get(1).(flow.BlockStatus)
	}

	if rf, ok := ret.Get(2).(func(context.Context, bool) error); ok {
		r2 = rf(ctx, isSealed)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// GetLatestProtocolStateSnapshot provides a mock function with given fields: ctx
func (_m *API) GetLatestProtocolStateSnapshot(ctx context.Context) ([]byte, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetLatestProtocolStateSnapshot")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) ([]byte, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) []byte); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNetworkParameters provides a mock function with given fields: ctx
func (_m *API) GetNetworkParameters(ctx context.Context) access.NetworkParameters {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetNetworkParameters")
	}

	var r0 access.NetworkParameters
	if rf, ok := ret.Get(0).(func(context.Context) access.NetworkParameters); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Get(0).(access.NetworkParameters)
	}

	return r0
}

// GetNodeVersionInfo provides a mock function with given fields: ctx
func (_m *API) GetNodeVersionInfo(ctx context.Context) (*access.NodeVersionInfo, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for GetNodeVersionInfo")
	}

	var r0 *access.NodeVersionInfo
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context) (*access.NodeVersionInfo, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *access.NodeVersionInfo); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.NodeVersionInfo)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetProtocolStateSnapshotByBlockID provides a mock function with given fields: ctx, blockID
func (_m *API) GetProtocolStateSnapshotByBlockID(ctx context.Context, blockID flow.Identifier) ([]byte, error) {
	ret := _m.Called(ctx, blockID)

	if len(ret) == 0 {
		panic("no return value specified for GetProtocolStateSnapshotByBlockID")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) ([]byte, error)); ok {
		return rf(ctx, blockID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) []byte); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetProtocolStateSnapshotByHeight provides a mock function with given fields: ctx, blockHeight
func (_m *API) GetProtocolStateSnapshotByHeight(ctx context.Context, blockHeight uint64) ([]byte, error) {
	ret := _m.Called(ctx, blockHeight)

	if len(ret) == 0 {
		panic("no return value specified for GetProtocolStateSnapshotByHeight")
	}

	var r0 []byte
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, uint64) ([]byte, error)); ok {
		return rf(ctx, blockHeight)
	}
	if rf, ok := ret.Get(0).(func(context.Context, uint64) []byte); ok {
		r0 = rf(ctx, blockHeight)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, uint64) error); ok {
		r1 = rf(ctx, blockHeight)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSystemTransaction provides a mock function with given fields: ctx, blockID
func (_m *API) GetSystemTransaction(ctx context.Context, blockID flow.Identifier) (*flow.TransactionBody, error) {
	ret := _m.Called(ctx, blockID)

	if len(ret) == 0 {
		panic("no return value specified for GetSystemTransaction")
	}

	var r0 *flow.TransactionBody
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.TransactionBody, error)); ok {
		return rf(ctx, blockID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.TransactionBody); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.TransactionBody)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSystemTransactionResult provides a mock function with given fields: ctx, blockID, requiredEventEncodingVersion
func (_m *API) GetSystemTransactionResult(ctx context.Context, blockID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) (*access.TransactionResult, error) {
	ret := _m.Called(ctx, blockID, requiredEventEncodingVersion)

	if len(ret) == 0 {
		panic("no return value specified for GetSystemTransactionResult")
	}

	var r0 *access.TransactionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, entities.EventEncodingVersion) (*access.TransactionResult, error)); ok {
		return rf(ctx, blockID, requiredEventEncodingVersion)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, entities.EventEncodingVersion) *access.TransactionResult); ok {
		r0 = rf(ctx, blockID, requiredEventEncodingVersion)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier, entities.EventEncodingVersion) error); ok {
		r1 = rf(ctx, blockID, requiredEventEncodingVersion)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransaction provides a mock function with given fields: ctx, id
func (_m *API) GetTransaction(ctx context.Context, id flow.Identifier) (*flow.TransactionBody, error) {
	ret := _m.Called(ctx, id)

	if len(ret) == 0 {
		panic("no return value specified for GetTransaction")
	}

	var r0 *flow.TransactionBody
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) (*flow.TransactionBody, error)); ok {
		return rf(ctx, id)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) *flow.TransactionBody); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*flow.TransactionBody)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionResult provides a mock function with given fields: ctx, id, blockID, collectionID, requiredEventEncodingVersion
func (_m *API) GetTransactionResult(ctx context.Context, id flow.Identifier, blockID flow.Identifier, collectionID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) (*access.TransactionResult, error) {
	ret := _m.Called(ctx, id, blockID, collectionID, requiredEventEncodingVersion)

	if len(ret) == 0 {
		panic("no return value specified for GetTransactionResult")
	}

	var r0 *access.TransactionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, flow.Identifier, flow.Identifier, entities.EventEncodingVersion) (*access.TransactionResult, error)); ok {
		return rf(ctx, id, blockID, collectionID, requiredEventEncodingVersion)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, flow.Identifier, flow.Identifier, entities.EventEncodingVersion) *access.TransactionResult); ok {
		r0 = rf(ctx, id, blockID, collectionID, requiredEventEncodingVersion)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier, flow.Identifier, flow.Identifier, entities.EventEncodingVersion) error); ok {
		r1 = rf(ctx, id, blockID, collectionID, requiredEventEncodingVersion)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionResultByIndex provides a mock function with given fields: ctx, blockID, index, requiredEventEncodingVersion
func (_m *API) GetTransactionResultByIndex(ctx context.Context, blockID flow.Identifier, index uint32, requiredEventEncodingVersion entities.EventEncodingVersion) (*access.TransactionResult, error) {
	ret := _m.Called(ctx, blockID, index, requiredEventEncodingVersion)

	if len(ret) == 0 {
		panic("no return value specified for GetTransactionResultByIndex")
	}

	var r0 *access.TransactionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, uint32, entities.EventEncodingVersion) (*access.TransactionResult, error)); ok {
		return rf(ctx, blockID, index, requiredEventEncodingVersion)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, uint32, entities.EventEncodingVersion) *access.TransactionResult); ok {
		r0 = rf(ctx, blockID, index, requiredEventEncodingVersion)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier, uint32, entities.EventEncodingVersion) error); ok {
		r1 = rf(ctx, blockID, index, requiredEventEncodingVersion)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionResultsByBlockID provides a mock function with given fields: ctx, blockID, requiredEventEncodingVersion
func (_m *API) GetTransactionResultsByBlockID(ctx context.Context, blockID flow.Identifier, requiredEventEncodingVersion entities.EventEncodingVersion) ([]*access.TransactionResult, error) {
	ret := _m.Called(ctx, blockID, requiredEventEncodingVersion)

	if len(ret) == 0 {
		panic("no return value specified for GetTransactionResultsByBlockID")
	}

	var r0 []*access.TransactionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, entities.EventEncodingVersion) ([]*access.TransactionResult, error)); ok {
		return rf(ctx, blockID, requiredEventEncodingVersion)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier, entities.EventEncodingVersion) []*access.TransactionResult); ok {
		r0 = rf(ctx, blockID, requiredEventEncodingVersion)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*access.TransactionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier, entities.EventEncodingVersion) error); ok {
		r1 = rf(ctx, blockID, requiredEventEncodingVersion)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionsByBlockID provides a mock function with given fields: ctx, blockID
func (_m *API) GetTransactionsByBlockID(ctx context.Context, blockID flow.Identifier) ([]*flow.TransactionBody, error) {
	ret := _m.Called(ctx, blockID)

	if len(ret) == 0 {
		panic("no return value specified for GetTransactionsByBlockID")
	}

	var r0 []*flow.TransactionBody
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) ([]*flow.TransactionBody, error)); ok {
		return rf(ctx, blockID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, flow.Identifier) []*flow.TransactionBody); ok {
		r0 = rf(ctx, blockID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*flow.TransactionBody)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, flow.Identifier) error); ok {
		r1 = rf(ctx, blockID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ping provides a mock function with given fields: ctx
func (_m *API) Ping(ctx context.Context) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Ping")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendTransaction provides a mock function with given fields: ctx, tx
func (_m *API) SendTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	ret := _m.Called(ctx, tx)

	if len(ret) == 0 {
		panic("no return value specified for SendTransaction")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *flow.TransactionBody) error); ok {
		r0 = rf(ctx, tx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewAPI creates a new instance of API. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewAPI(t interface {
	mock.TestingT
	Cleanup(func())
}) *API {
	mock := &API{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
