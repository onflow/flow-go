package execution

import (
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
)

// CollectionExecutionResult holds aggregated artifacts (events, tx resutls, ...)
// generated during collection execution
type CollectionExecutionResult struct {
	events                 flow.EventsList
	serviceEvents          flow.EventsList
	convertedServiceEvents flow.ServiceEventList
	transactionResults     flow.TransactionResults
	executionSnapshot      *snapshot.ExecutionSnapshot
}

// NewEmptyCollectionExecutionResult constructs a new  CollectionExecutionResult
func NewEmptyCollectionExecutionResult() *CollectionExecutionResult {
	return &CollectionExecutionResult{
		events:                 make(flow.EventsList, 0),
		serviceEvents:          make(flow.EventsList, 0),
		convertedServiceEvents: make(flow.ServiceEventList, 0),
		transactionResults:     make(flow.TransactionResults, 0),
	}
}

func (c *CollectionExecutionResult) AppendTransactionResults(
	events flow.EventsList,
	serviceEvents flow.EventsList,
	convertedServiceEvents flow.ServiceEventList,
	transactionResult flow.TransactionResult,
) {
	c.events = append(c.events, events...)
	c.serviceEvents = append(c.serviceEvents, serviceEvents...)
	c.convertedServiceEvents = append(c.convertedServiceEvents, convertedServiceEvents...)
	c.transactionResults = append(c.transactionResults, transactionResult)
}

func (c *CollectionExecutionResult) UpdateExecutionSnapshot(
	executionSnapshot *snapshot.ExecutionSnapshot,
) {
	c.executionSnapshot = executionSnapshot
}

func (c *CollectionExecutionResult) ExecutionSnapshot() *snapshot.ExecutionSnapshot {
	return c.executionSnapshot
}

func (c *CollectionExecutionResult) Events() flow.EventsList {
	return c.events
}

func (c *CollectionExecutionResult) ServiceEventList() flow.EventsList {
	return c.serviceEvents
}

func (c *CollectionExecutionResult) ConvertedServiceEvents() flow.ServiceEventList {
	return c.convertedServiceEvents
}

func (c *CollectionExecutionResult) TransactionResults() flow.TransactionResults {
	return c.transactionResults
}

// CollectionAttestationResult holds attestations generated during post-processing
// phase of collect execution.
type CollectionAttestationResult struct {
	startStateCommit flow.StateCommitment
	endStateCommit   flow.StateCommitment
	stateProof       flow.StorageProof
	eventCommit      flow.Identifier
}

func NewCollectionAttestationResult(
	startStateCommit flow.StateCommitment,
	endStateCommit flow.StateCommitment,
	stateProof flow.StorageProof,
	eventCommit flow.Identifier,
) *CollectionAttestationResult {
	return &CollectionAttestationResult{
		startStateCommit: startStateCommit,
		endStateCommit:   endStateCommit,
		stateProof:       stateProof,
		eventCommit:      eventCommit,
	}
}

func (a *CollectionAttestationResult) StartStateCommitment() flow.StateCommitment {
	return a.startStateCommit
}

func (a *CollectionAttestationResult) EndStateCommitment() flow.StateCommitment {
	return a.endStateCommit
}

func (a *CollectionAttestationResult) StateProof() flow.StorageProof {
	return a.stateProof
}

func (a *CollectionAttestationResult) EventCommitment() flow.Identifier {
	return a.eventCommit
}

// TODO(ramtin): depricate in the future, temp method, needed for uploader for now
func (a *CollectionAttestationResult) UpdateEndStateCommitment(endState flow.StateCommitment) {
	a.endStateCommit = endState
}
