package requester

import (
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/state_synchronization"
)

func WithCollections(collections []*flow.Collection) func(*state_synchronization.ExecutionData) {
	return func(executionData *state_synchronization.ExecutionData) {
		executionData.Collections = collections
	}
}

func WithEvents(events []flow.EventsList) func(*state_synchronization.ExecutionData) {
	return func(executionData *state_synchronization.ExecutionData) {
		executionData.Events = events
	}
}

func WithTrieUpdates(updates []*ledger.TrieUpdate) func(*state_synchronization.ExecutionData) {
	return func(executionData *state_synchronization.ExecutionData) {
		executionData.TrieUpdates = updates
	}
}

func ExecutionDataFixture(blockID flow.Identifier) *state_synchronization.ExecutionData {
	return &state_synchronization.ExecutionData{
		BlockID:     blockID,
		Collections: []*flow.Collection{},
		Events:      []flow.EventsList{},
		TrieUpdates: []*ledger.TrieUpdate{},
	}
}
