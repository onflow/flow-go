package hotstuff

import "github.com/onflow/flow-go/consensus/hotstuff/model"

// VotesQueue is a queue to buffer the votes to be processed by multiple workers concurrently.
type VotesQueue interface {
	// Push pushes a vote to the queue and notify an idle worker to process it if available.
	// It returns true when the vote was added to the queue.
	// It returns false when the queue is full and the vote was not added to the queue.
	Push(vote *model.Vote) bool

	// Consume create the given number of workers to process votes in the queue concurrently.
	// the given consumer function will be called to create a worker and process the vote.
	// the given done channel will stop from processing more votes.
	// This function blocks until it receives a signal from the done channel
	Consume(nWorker int, consumer func(vote *model.Vote) error, done chan<- interface{}) error
}

type Worker interface {
	// ProcessVote processes each vote
	ProcessVote(vote *model.Vote) error
}
