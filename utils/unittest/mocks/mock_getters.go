package mocks

import "github.com/onflow/flow-go/storage"

// StorageMapGetter implements a simple generic getter function for mock storage methods.
// This is useful to avoid duplicating boilerplate code for mock storage methods.
//
// Example:
// Instead of the following code:
//
//	results.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
//		func(resultID flow.Identifier) (*flow.ExecutionResult, error) {
//			if result, ok := s.resultMap[resultID]; ok {
//				return result, nil
//			}
//			return nil, storage.ErrNotFound
//		},
//	)
//
// Use this:
//
//	results.On("ByID", mock.AnythingOfType("flow.Identifier")).Return(
//		mocks.StorageMapGetter(s.resultMap),
//	)
func StorageMapGetter[K comparable, V any](m map[K]V) func(key K) (V, error) {
	return func(key K) (V, error) {
		if val, ok := m[key]; ok {
			return val, nil
		}
		return *new(V), storage.ErrNotFound
	}
}

// ConvertStorageOutput maps the output type from a getter function to a different type.
// This is useful to avoid maintaining multiple maps for the same data.
//
// Example usage:
//
//	blockMap := map[uint64]*flow.Block{}
//
//	headers.On("BlockIDByHeight", mock.AnythingOfType("uint64")).Return(
//		mocks.ConvertStorageOutput(
//			mocks.StorageMapGetter(s.blockMap),
//			func(block *flow.Block) flow.Identifier { return block.ID() },
//		),
//	)
func ConvertStorageOutput[K comparable, V any, R any](fn func(key K) (V, error), mapper func(V) R) func(key K) (R, error) {
	return func(key K) (R, error) {
		v, err := fn(key)
		if err != nil {
			return *new(R), err
		}
		return mapper(v), err
	}
}
