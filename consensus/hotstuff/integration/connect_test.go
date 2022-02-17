package integration

import (
	"fmt"
	"time"

	"github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
)

func Connect(instances []*Instance) {

	// first, create a map of all instances and a queue for each
	lookup := make(map[flow.Identifier]*Instance)
	for _, in := range instances {
		lookup[in.localID] = in
	}

	// then, for each instance, initialize a wired up communicator
	for _, sender := range instances {
		sender := sender // avoid capturing loop variable in closure

		*sender.communicator = mocks.Communicator{}
		sender.communicator.On("BroadcastProposalWithDelay", mock.Anything, mock.Anything).Return(
			func(header *flow.Header, delay time.Duration) error {

				// sender should always have the parent
				parent, exists := sender.headers[header.ParentID]
				if !exists {
					return fmt.Errorf("parent for proposal not found (sender: %x, parent: %x)", sender.localID, header.ParentID)
				}

				// fill in the header chain ID and height
				header.ChainID = parent.ChainID
				header.Height = parent.Height + 1

				// convert into proposal immediately
				proposal := model.ProposalFromFlow(header, parent.View)

				// store locally and loop back to engine for processing
				sender.ProcessBlock(proposal)

				// check if we should block the outgoing proposal
				if sender.blockPropOut(proposal) {
					return nil
				}

				// iterate through potential receivers
				for _, receiver := range instances {

					// we should skip ourselves always
					if receiver.localID == sender.localID {
						continue
					}

					// check if we should block the incoming proposal
					if receiver.blockPropIn(proposal) {
						return nil
					}

					receiver.ProcessBlock(proposal)
				}

				return nil
			},
		)
		sender.communicator.On("SendVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			func(blockID flow.Identifier, view uint64, sigData []byte, recipientID flow.Identifier) error {

				// convert into vote
				vote := model.VoteFromFlow(sender.localID, blockID, view, sigData)

				// should never send to self
				if recipientID == sender.localID {
					return fmt.Errorf("can't send to self (sender: %x)", sender.localID)
				}

				// check if we should block the outgoing vote
				if sender.blockVoteOut(vote) {
					return nil
				}

				// get the receiver
				receiver, exists := lookup[recipientID]
				if !exists {
					return fmt.Errorf("recipient doesn't exist (sender: %x, receiver: %x)", sender.localID, recipientID)
				}

				// check if e should block the incoming vote
				if receiver.blockVoteIn(vote) {
					return nil
				}

				// submit the vote to the receiving event loop (non-blocking)
				receiver.queue <- vote

				return nil
			},
		)
	}
}
