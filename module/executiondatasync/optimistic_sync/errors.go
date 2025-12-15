package optimistic_sync

import (
	"errors"
)

// ErrForkAbandoned is returned if the execution fork of an execution node from which we were getting the execution
// results was abandoned.
var ErrForkAbandoned = errors.New("current execution fork has been abandoned")

// ErrRequiredExecutorNotFound is returned if the criteria's required executor is not in the group of execution nodes
// that produced the execution result.
var ErrRequiredExecutorNotFound = errors.New("required executor not found")

// ErrNotEnoughAgreeingExecutors is returned if there are not enough execution nodes that produced the execution result.
var ErrNotEnoughAgreeingExecutors = errors.New("not enough agreeing executors found")

// ErrBlockBeforeNodeHistory is returned when the requested block predates what the node has in storage
// (for example, requesting the spork root block while the node was bootstrapped from a newer block).
var ErrBlockBeforeNodeHistory = errors.New("requested block is before node history")
