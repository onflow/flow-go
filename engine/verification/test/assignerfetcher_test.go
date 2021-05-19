package test

import (
	"testing"

	vertestutils "github.com/onflow/flow-go/engine/verification/utils/unittest"
	"github.com/onflow/flow-go/module/metrics"
)

// TestVerificationHappyPath evaluates behavior of the pipeline of verification node engines as:
// block reader -> block consumer -> assigner engine -> chunks queue -> chunks consumer -> fetcher engine -> verifier engine
// block reader receives (container) finalized blocks that contain execution receipts preceding (reference) blocks.
// some receipts have duplicate results.
// - in a staked verification node:
// -- in assigner engine, for each distinct result it receives:
// --- it does the chunk assignment.
// --- it passes the chunk locators of assigned chunks to chunk queue.
// --- the chunk queue in turn delivers the assigned chunk to the fetcher engine.
// -- in fetcher engine, for each arriving chunk locator:
// --- it asks the chunk data pack from requester engine.
// --- requester engine asks and retrieves chunk data pack from (mocked) execution node.
// --- once chunk data pack arrives, forms a verifiable chunk and passes it to verifier node.
// -- in verifier engine, for each arriving verifiable chunk:
// --- it verifies the chunk, shapes a result approval, and emits it to (mock) consensus node.
// -- the test is passed if (mock) consensus node receives a single result approval per assigned chunk in a timely manner.
// - in an unstaked verification node:
// -- execution results are discarded.
// -- the test is passed if no result approval is emitted for any of the chunks in a timely manner.
func TestVerificationHappyPath(t *testing.T) {
	testcases := []struct {
		blockCount      int
		opts            []vertestutils.CompleteExecutionReceiptBuilderOpt
		msg             string
		staked          bool
		eventRepetition int // accounts for consumer being notified of a certain finalized block more than once.
	}{
		{
			// read this test case in this way:
			// one block is passed to block reader. The block contains one
			// execution result that is not duplicate (single copy).
			// The result has only one chunk.
			// The verification node is staked
			blockCount: 1,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(1),
				vertestutils.WithChunksCount(1),
				vertestutils.WithCopies(1),
			},
			staked:          true,
			eventRepetition: 1,
			msg:             "1 block, 1 result, 1 chunk, no duplicate, staked, no event repetition",
		},
		{
			blockCount: 1,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(1),
				vertestutils.WithChunksCount(1),
				vertestutils.WithCopies(1),
			},
			staked:          false, // unstaked
			eventRepetition: 1,
			msg:             "1 block, 1 result, 1 chunk, no duplicate, unstaked, no event repetition",
		},
		{
			blockCount: 1,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(5),
				vertestutils.WithChunksCount(5),
				vertestutils.WithCopies(1),
			},
			staked:          true,
			eventRepetition: 1,
			msg:             "1 block, 5 result, 5 chunks, no duplicate, staked, no event repetition",
		},
		{
			blockCount: 10,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(2),
				vertestutils.WithChunksCount(2),
				vertestutils.WithCopies(2),
			},
			staked:          true,
			eventRepetition: 1,
			msg:             "10 block, 5 result, 5 chunks, 1 duplicates, staked, no event repetition",
		},
		{
			blockCount: 10,
			opts: []vertestutils.CompleteExecutionReceiptBuilderOpt{
				vertestutils.WithResults(2),
				vertestutils.WithChunksCount(2),
				vertestutils.WithCopies(2),
			},
			staked:          true,
			eventRepetition: 3, // notifies consumer 3 times for each finalized block.
			msg:             "10 block, 5 result, 5 chunks, 1 duplicates, staked, with event repetition",
		},
	}

	for _, tc := range testcases {
		t.Run(tc.msg, func(t *testing.T) {
			collector := &metrics.NoopCollector{}

			vertestutils.NewVerificationHappyPathTest(t,
				tc.staked,
				tc.blockCount,
				tc.eventRepetition,
				collector,
				collector,
				tc.opts...)
		})
	}
}
