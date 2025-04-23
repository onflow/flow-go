package recovery

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRecover(t *testing.T) {
	finalized := unittest.BlockHeaderFixture()
	proposals := unittest.ProposalChainFixtureFrom(100, finalized)
	pending := make([]*flow.ProposalHeader, 0)
	for _, b := range proposals {
		pending = append(pending, b.HeaderProposal())
	}

	// Recover with `pending` blocks and record what blocks are forwarded to `onProposal`
	recovered := make([]*model.SignedProposal, 0)
	scanner := func(block *model.SignedProposal) error {
		recovered = append(recovered, block)
		return nil
	}
	err := Recover(unittest.Logger(), pending, scanner)
	require.NoError(t, err)

	// should forward blocks in exact order, just converting flow.ProposalHeader to pending block
	require.Len(t, recovered, len(pending))
	for i, r := range recovered {
		require.Equal(t, model.SignedProposalFromFlow(pending[i]), r)
	}
}

func TestRecoverEmptyInput(t *testing.T) {
	scanner := func(block *model.SignedProposal) error {
		require.Fail(t, "no proposal expected")
		return nil
	}
	err := Recover(unittest.Logger(), []*flow.ProposalHeader{}, scanner)
	require.NoError(t, err)
}

func TestCollector(t *testing.T) {
	t.Run("empty retrieve", func(t *testing.T) {
		c := NewCollector[string]()
		require.Empty(t, c.Retrieve())
	})

	t.Run("append", func(t *testing.T) {
		c := NewCollector[string]()
		strings := []string{"a", "b", "c"}
		appended := 0
		for _, s := range strings {
			c.Append(s)
			appended++
			require.Equal(t, strings[:appended], c.Retrieve())
		}
	})

	t.Run("append multiple", func(t *testing.T) {
		c := NewCollector[string]()
		strings := []string{"a", "b", "c", "d", "e"}

		c.Append(strings[0], strings[1])
		require.Equal(t, strings[:2], c.Retrieve())

		c.Append(strings[2], strings[3], strings[4])
		require.Equal(t, strings, c.Retrieve())
	})

	t.Run("safely passed by value", func(t *testing.T) {
		strings := []string{"a", "b"}
		c := NewCollector[string]()
		c.Append(strings[0])

		// pass by value
		c2 := c
		require.Equal(t, strings[:1], c2.Retrieve())

		// add to original; change could be reflected by c2:
		c.Append(strings[1])
		require.Equal(t, strings, c2.Retrieve())
	})

	t.Run("append after retrieve", func(t *testing.T) {
		c := NewCollector[string]()
		strings := []string{"a", "b", "c", "d", "e"}

		c.Append(strings[0], strings[1])
		retrieved := c.Retrieve()
		require.Equal(t, strings[:2], retrieved)

		// appending further elements shouldn't affect previously retrieved list
		c.Append(strings[2], strings[3], strings[4])
		require.Equal(t, strings[:2], retrieved)
		require.Equal(t, strings, c.Retrieve())
	})
}
