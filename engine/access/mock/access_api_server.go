// Code generated by mockery v2.21.4. DO NOT EDIT.

package mock

import (
	context "context"

	access "github.com/onflow/flow/protobuf/go/flow/access"

	mock "github.com/stretchr/testify/mock"
)

// AccessAPIServer is an autogenerated mock type for the AccessAPIServer type
type AccessAPIServer struct {
	mock.Mock
}

// ExecuteScriptAtBlockHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) ExecuteScriptAtBlockHeight(_a0 context.Context, _a1 *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ExecuteScriptResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.ExecuteScriptAtBlockHeightRequest) (*access.ExecuteScriptResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.ExecuteScriptAtBlockHeightRequest) *access.ExecuteScriptResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ExecuteScriptResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.ExecuteScriptAtBlockHeightRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteScriptAtBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) ExecuteScriptAtBlockID(_a0 context.Context, _a1 *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ExecuteScriptResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.ExecuteScriptAtBlockIDRequest) (*access.ExecuteScriptResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.ExecuteScriptAtBlockIDRequest) *access.ExecuteScriptResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ExecuteScriptResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.ExecuteScriptAtBlockIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ExecuteScriptAtLatestBlock provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) ExecuteScriptAtLatestBlock(_a0 context.Context, _a1 *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ExecuteScriptResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.ExecuteScriptAtLatestBlockRequest) (*access.ExecuteScriptResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.ExecuteScriptAtLatestBlockRequest) *access.ExecuteScriptResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ExecuteScriptResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.ExecuteScriptAtLatestBlockRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccount provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetAccount(_a0 context.Context, _a1 *access.GetAccountRequest) (*access.GetAccountResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.GetAccountResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetAccountRequest) (*access.GetAccountResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetAccountRequest) *access.GetAccountResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.GetAccountResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetAccountRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccountAtBlockHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetAccountAtBlockHeight(_a0 context.Context, _a1 *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.AccountResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetAccountAtBlockHeightRequest) (*access.AccountResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetAccountAtBlockHeightRequest) *access.AccountResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.AccountResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetAccountAtBlockHeightRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetAccountAtLatestBlock provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetAccountAtLatestBlock(_a0 context.Context, _a1 *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.AccountResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetAccountAtLatestBlockRequest) (*access.AccountResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetAccountAtLatestBlockRequest) *access.AccountResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.AccountResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetAccountAtLatestBlockRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockByHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetBlockByHeight(_a0 context.Context, _a1 *access.GetBlockByHeightRequest) (*access.BlockResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.BlockResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockByHeightRequest) (*access.BlockResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockByHeightRequest) *access.BlockResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.BlockResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetBlockByHeightRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockByID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetBlockByID(_a0 context.Context, _a1 *access.GetBlockByIDRequest) (*access.BlockResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.BlockResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockByIDRequest) (*access.BlockResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockByIDRequest) *access.BlockResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.BlockResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetBlockByIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockHeaderByHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetBlockHeaderByHeight(_a0 context.Context, _a1 *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.BlockHeaderResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockHeaderByHeightRequest) (*access.BlockHeaderResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockHeaderByHeightRequest) *access.BlockHeaderResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.BlockHeaderResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetBlockHeaderByHeightRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetBlockHeaderByID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetBlockHeaderByID(_a0 context.Context, _a1 *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.BlockHeaderResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockHeaderByIDRequest) (*access.BlockHeaderResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetBlockHeaderByIDRequest) *access.BlockHeaderResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.BlockHeaderResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetBlockHeaderByIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetCollectionByID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetCollectionByID(_a0 context.Context, _a1 *access.GetCollectionByIDRequest) (*access.CollectionResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.CollectionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetCollectionByIDRequest) (*access.CollectionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetCollectionByIDRequest) *access.CollectionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.CollectionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetCollectionByIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEventsForBlockIDs provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetEventsForBlockIDs(_a0 context.Context, _a1 *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.EventsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetEventsForBlockIDsRequest) (*access.EventsResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetEventsForBlockIDsRequest) *access.EventsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.EventsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetEventsForBlockIDsRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetEventsForHeightRange provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetEventsForHeightRange(_a0 context.Context, _a1 *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.EventsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetEventsForHeightRangeRequest) (*access.EventsResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetEventsForHeightRangeRequest) *access.EventsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.EventsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetEventsForHeightRangeRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutionResultByID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetExecutionResultByID(_a0 context.Context, _a1 *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ExecutionResultByIDResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetExecutionResultByIDRequest) (*access.ExecutionResultByIDResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetExecutionResultByIDRequest) *access.ExecutionResultByIDResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ExecutionResultByIDResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetExecutionResultByIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetExecutionResultForBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetExecutionResultForBlockID(_a0 context.Context, _a1 *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ExecutionResultForBlockIDResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetExecutionResultForBlockIDRequest) (*access.ExecutionResultForBlockIDResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetExecutionResultForBlockIDRequest) *access.ExecutionResultForBlockIDResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ExecutionResultForBlockIDResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetExecutionResultForBlockIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetFullCollectionByID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetFullCollectionByID(_a0 context.Context, _a1 *access.GetFullCollectionByIDRequest) (*access.FullCollectionResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.FullCollectionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetFullCollectionByIDRequest) (*access.FullCollectionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetFullCollectionByIDRequest) *access.FullCollectionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.FullCollectionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetFullCollectionByIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestBlock provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetLatestBlock(_a0 context.Context, _a1 *access.GetLatestBlockRequest) (*access.BlockResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.BlockResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetLatestBlockRequest) (*access.BlockResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetLatestBlockRequest) *access.BlockResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.BlockResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetLatestBlockRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestBlockHeader provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetLatestBlockHeader(_a0 context.Context, _a1 *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.BlockHeaderResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetLatestBlockHeaderRequest) (*access.BlockHeaderResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetLatestBlockHeaderRequest) *access.BlockHeaderResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.BlockHeaderResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetLatestBlockHeaderRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetLatestProtocolStateSnapshot provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetLatestProtocolStateSnapshot(_a0 context.Context, _a1 *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ProtocolStateSnapshotResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetLatestProtocolStateSnapshotRequest) (*access.ProtocolStateSnapshotResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetLatestProtocolStateSnapshotRequest) *access.ProtocolStateSnapshotResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ProtocolStateSnapshotResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetLatestProtocolStateSnapshotRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNetworkParameters provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetNetworkParameters(_a0 context.Context, _a1 *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.GetNetworkParametersResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetNetworkParametersRequest) (*access.GetNetworkParametersResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetNetworkParametersRequest) *access.GetNetworkParametersResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.GetNetworkParametersResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetNetworkParametersRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetNodeVersionInfo provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetNodeVersionInfo(_a0 context.Context, _a1 *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.GetNodeVersionInfoResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetNodeVersionInfoRequest) (*access.GetNodeVersionInfoResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetNodeVersionInfoRequest) *access.GetNodeVersionInfoResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.GetNodeVersionInfoResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetNodeVersionInfoRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetProtocolStateSnapshotByBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetProtocolStateSnapshotByBlockID(_a0 context.Context, _a1 *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ProtocolStateSnapshotResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetProtocolStateSnapshotByBlockIDRequest) (*access.ProtocolStateSnapshotResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetProtocolStateSnapshotByBlockIDRequest) *access.ProtocolStateSnapshotResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ProtocolStateSnapshotResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetProtocolStateSnapshotByBlockIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetProtocolStateSnapshotByHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetProtocolStateSnapshotByHeight(_a0 context.Context, _a1 *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.ProtocolStateSnapshotResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetProtocolStateSnapshotByHeightRequest) (*access.ProtocolStateSnapshotResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetProtocolStateSnapshotByHeightRequest) *access.ProtocolStateSnapshotResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.ProtocolStateSnapshotResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetProtocolStateSnapshotByHeightRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSystemTransaction provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetSystemTransaction(_a0 context.Context, _a1 *access.GetSystemTransactionRequest) (*access.TransactionResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.TransactionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetSystemTransactionRequest) (*access.TransactionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetSystemTransactionRequest) *access.TransactionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetSystemTransactionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetSystemTransactionResult provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetSystemTransactionResult(_a0 context.Context, _a1 *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.TransactionResultResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetSystemTransactionResultRequest) (*access.TransactionResultResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetSystemTransactionResultRequest) *access.TransactionResultResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResultResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetSystemTransactionResultRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransaction provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetTransaction(_a0 context.Context, _a1 *access.GetTransactionRequest) (*access.TransactionResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.TransactionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionRequest) (*access.TransactionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionRequest) *access.TransactionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetTransactionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionResult provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetTransactionResult(_a0 context.Context, _a1 *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.TransactionResultResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionRequest) (*access.TransactionResultResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionRequest) *access.TransactionResultResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResultResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetTransactionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionResultByIndex provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetTransactionResultByIndex(_a0 context.Context, _a1 *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.TransactionResultResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionByIndexRequest) (*access.TransactionResultResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionByIndexRequest) *access.TransactionResultResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResultResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetTransactionByIndexRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionResultsByBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetTransactionResultsByBlockID(_a0 context.Context, _a1 *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.TransactionResultsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionsByBlockIDRequest) (*access.TransactionResultsResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionsByBlockIDRequest) *access.TransactionResultsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionResultsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetTransactionsByBlockIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetTransactionsByBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) GetTransactionsByBlockID(_a0 context.Context, _a1 *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.TransactionsResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionsByBlockIDRequest) (*access.TransactionsResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.GetTransactionsByBlockIDRequest) *access.TransactionsResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.TransactionsResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.GetTransactionsByBlockIDRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// Ping provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) Ping(_a0 context.Context, _a1 *access.PingRequest) (*access.PingResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.PingResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.PingRequest) (*access.PingResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.PingRequest) *access.PingResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.PingResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.PingRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SendAndSubscribeTransactionStatuses provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SendAndSubscribeTransactionStatuses(_a0 *access.SendAndSubscribeTransactionStatusesRequest, _a1 access.AccessAPI_SendAndSubscribeTransactionStatusesServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SendAndSubscribeTransactionStatusesRequest, access.AccessAPI_SendAndSubscribeTransactionStatusesServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SendTransaction provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SendTransaction(_a0 context.Context, _a1 *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {
	ret := _m.Called(_a0, _a1)

	var r0 *access.SendTransactionResponse
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *access.SendTransactionRequest) (*access.SendTransactionResponse, error)); ok {
		return rf(_a0, _a1)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *access.SendTransactionRequest) *access.SendTransactionResponse); ok {
		r0 = rf(_a0, _a1)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*access.SendTransactionResponse)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *access.SendTransactionRequest) error); ok {
		r1 = rf(_a0, _a1)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// SubscribeBlockDigestsFromLatest provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlockDigestsFromLatest(_a0 *access.SubscribeBlockDigestsFromLatestRequest, _a1 access.AccessAPI_SubscribeBlockDigestsFromLatestServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlockDigestsFromLatestRequest, access.AccessAPI_SubscribeBlockDigestsFromLatestServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlockDigestsFromStartBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlockDigestsFromStartBlockID(_a0 *access.SubscribeBlockDigestsFromStartBlockIDRequest, _a1 access.AccessAPI_SubscribeBlockDigestsFromStartBlockIDServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlockDigestsFromStartBlockIDRequest, access.AccessAPI_SubscribeBlockDigestsFromStartBlockIDServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlockDigestsFromStartHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlockDigestsFromStartHeight(_a0 *access.SubscribeBlockDigestsFromStartHeightRequest, _a1 access.AccessAPI_SubscribeBlockDigestsFromStartHeightServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlockDigestsFromStartHeightRequest, access.AccessAPI_SubscribeBlockDigestsFromStartHeightServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlockHeadersFromLatest provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlockHeadersFromLatest(_a0 *access.SubscribeBlockHeadersFromLatestRequest, _a1 access.AccessAPI_SubscribeBlockHeadersFromLatestServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlockHeadersFromLatestRequest, access.AccessAPI_SubscribeBlockHeadersFromLatestServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlockHeadersFromStartBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlockHeadersFromStartBlockID(_a0 *access.SubscribeBlockHeadersFromStartBlockIDRequest, _a1 access.AccessAPI_SubscribeBlockHeadersFromStartBlockIDServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlockHeadersFromStartBlockIDRequest, access.AccessAPI_SubscribeBlockHeadersFromStartBlockIDServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlockHeadersFromStartHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlockHeadersFromStartHeight(_a0 *access.SubscribeBlockHeadersFromStartHeightRequest, _a1 access.AccessAPI_SubscribeBlockHeadersFromStartHeightServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlockHeadersFromStartHeightRequest, access.AccessAPI_SubscribeBlockHeadersFromStartHeightServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlocksFromLatest provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlocksFromLatest(_a0 *access.SubscribeBlocksFromLatestRequest, _a1 access.AccessAPI_SubscribeBlocksFromLatestServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlocksFromLatestRequest, access.AccessAPI_SubscribeBlocksFromLatestServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlocksFromStartBlockID provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlocksFromStartBlockID(_a0 *access.SubscribeBlocksFromStartBlockIDRequest, _a1 access.AccessAPI_SubscribeBlocksFromStartBlockIDServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlocksFromStartBlockIDRequest, access.AccessAPI_SubscribeBlocksFromStartBlockIDServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SubscribeBlocksFromStartHeight provides a mock function with given fields: _a0, _a1
func (_m *AccessAPIServer) SubscribeBlocksFromStartHeight(_a0 *access.SubscribeBlocksFromStartHeightRequest, _a1 access.AccessAPI_SubscribeBlocksFromStartHeightServer) error {
	ret := _m.Called(_a0, _a1)

	var r0 error
	if rf, ok := ret.Get(0).(func(*access.SubscribeBlocksFromStartHeightRequest, access.AccessAPI_SubscribeBlocksFromStartHeightServer) error); ok {
		r0 = rf(_a0, _a1)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

type mockConstructorTestingTNewAccessAPIServer interface {
	mock.TestingT
	Cleanup(func())
}

// NewAccessAPIServer creates a new instance of AccessAPIServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
func NewAccessAPIServer(t mockConstructorTestingTNewAccessAPIServer) *AccessAPIServer {
	mock := &AccessAPIServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
