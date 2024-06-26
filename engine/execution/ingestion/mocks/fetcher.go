package mocks

import "github.com/onflow/flow-go/model/flow"

type MockFetcher struct {
	byID map[flow.Identifier]struct{}
}

func NewMockFetcher() *MockFetcher {
	return &MockFetcher{
		byID: make(map[flow.Identifier]struct{}),
	}
}

func (r *MockFetcher) FetchCollection(blockID flow.Identifier, height uint64, guarantee *flow.CollectionGuarantee) error {
	r.byID[guarantee.ID()] = struct{}{}
	return nil
}

func (r *MockFetcher) Force() {
}

func (r *MockFetcher) IsFetched(id flow.Identifier) bool {
	_, ok := r.byID[id]
	return ok
}
