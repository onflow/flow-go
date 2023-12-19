package mocks

import "github.com/onflow/flow-go/storage"

// MockGetter implements a simple generic getter function for mock storage methods.
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
func StorageMapGetter[T comparable, R any](m map[T]R) func(key T) (R, error) {
	return func(key T) (R, error) {
		if val, ok := m[key]; ok {
			return val, nil
		}
		return *new(R), storage.ErrNotFound
	}
}
