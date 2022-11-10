package messages_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockProposal(t *testing.T) {
	block := unittest.FullBlockFixture()
	proposal := messages.NewBlockProposal(&block)
	converted := proposal.Block.ToInternal()
	assert.Equal(t, &block, converted)
}

func TestClusterBlockProposal(t *testing.T) {
	block := unittest.ClusterBlockFixture()
	proposal := messages.NewClusterBlockProposal(&block)
	converted := proposal.Block.ToInternal()
	assert.Equal(t, &block, converted)
}
