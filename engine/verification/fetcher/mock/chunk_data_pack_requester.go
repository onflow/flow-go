// Code generated by mockery v1.0.0. DO NOT EDIT.

package mockfetcher

import (
	fetcher "github.com/onflow/flow-go/engine/verification/fetcher"
	mock "github.com/stretchr/testify/mock"

	verification "github.com/onflow/flow-go/model/verification"
)

// ChunkDataPackRequester is an autogenerated mock type for the ChunkDataPackRequester type
type ChunkDataPackRequester struct {
	mock.Mock
}

// Request provides a mock function with given fields: request
func (_m *ChunkDataPackRequester) Request(request *verification.ChunkDataPackRequest) {
	_m.Called(request)
}

// WithChunkDataPackHandler provides a mock function with given fields: handler
func (_m *ChunkDataPackRequester) WithChunkDataPackHandler(handler fetcher.ChunkDataPackHandler) {
	_m.Called(handler)
}
