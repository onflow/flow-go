// Code generated by mockery v1.0.0. DO NOT EDIT.

package mockfetcher

import (
	verification "github.com/onflow/flow-go/model/verification"
	mock "github.com/stretchr/testify/mock"
)

// ChunkDataPackRequester is an autogenerated mock type for the ChunkDataPackRequester type
type ChunkDataPackRequester struct {
	mock.Mock
}

// Request provides a mock function with given fields: request
func (_m *ChunkDataPackRequester) Request(request *verification.ChunkDataPackRequest) {
	_m.Called(request)
}
