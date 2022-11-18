package integration

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

func Connect(t *testing.T, instances []*Instance) {

	// first, create a map of all instances and a queue for each
	lookup := make(map[flow.Identifier]*Instance)
	for _, in := range instances {
		lookup[in.localID] = in
	}

	// then, for each instance, initialize a wired up communicator
	for _, sender := range instances {
		sender := sender // avoid capturing loop variable in closure

		*sender.notifier = *NewMockedCommunicatorConsumer()
		sender.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(
			func(args mock.Arguments) {
				header, ok := args[0].(*flow.Header)
				require.True(t, ok)

				// sender should always have the parent
				sender.updatingBlocks.RLock()
				_, exists := sender.headers[header.ParentID]
				sender.updatingBlocks.RUnlock()
				if !exists {
					t.Fatalf("parent for proposal not found (sender: %x, parent: %x)", sender.localID, header.ParentID)
				}

				// convert into proposal immediately
				proposal := model.ProposalFromFlow(header)

				// store locally and loop back to engine for processing
				sender.ProcessBlock(proposal)

				// check if we should block the outgoing proposal
				if sender.blockPropOut(proposal) {
					return
				}

				// iterate through potential receivers
				for _, receiver := range instances {

					// we should skip ourselves always
					if receiver.localID == sender.localID {
						continue
					}

					// check if we should block the incoming proposal
					if receiver.blockPropIn(proposal) {
						continue
					}

					receiver.ProcessBlock(proposal)
				}
			},
		)
		sender.notifier.On("OnOwnVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(
			func(args mock.Arguments) {
				blockID, ok := args[0].(flow.Identifier)
				require.True(t, ok)
				view, ok := args[1].(uint64)
				require.True(t, ok)
				sigData, ok := args[2].([]byte)
				require.True(t, ok)
				recipientID, ok := args[3].(flow.Identifier)
				require.True(t, ok)
				// convert into vote
				vote := model.VoteFromFlow(sender.localID, blockID, view, sigData)

				// get the receiver
				receiver, exists := lookup[recipientID]
				if !exists {
					t.Fatalf("recipient doesn't exist (sender: %x, receiver: %x)", sender.localID, recipientID)
				}

				// if we are next leader we should be receiving our own vote
				if recipientID != sender.localID {
					// check if we should block the outgoing vote
					if sender.blockVoteOut(vote) {
						return
					}

					// check if e should block the incoming vote
					if receiver.blockVoteIn(vote) {
						return
					}
				}

				// submit the vote to the receiving event loop (non-blocking)
				receiver.queue <- vote
			},
		)
		sender.notifier.On("OnOwnTimeout", mock.Anything).Run(
			func(args mock.Arguments) {
				timeoutObject, ok := args[0].(*model.TimeoutObject)
				require.True(t, ok)
				// iterate through potential receivers
				for _, receiver := range instances {

					// we should skip ourselves always
					if receiver.localID == sender.localID {
						continue
					}

					// check if we should block the outgoing value
					if sender.blockTimeoutObjectOut(timeoutObject) {
						continue
					}

					// check if we should block the incoming value
					if receiver.blockTimeoutObjectIn(timeoutObject) {
						continue
					}

					receiver.queue <- timeoutObject
				}
			})
	}
}
