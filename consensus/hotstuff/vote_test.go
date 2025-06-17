package hotstuff

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestNewVote verifies that NewVote correctly constructs a Vote from valid input
// and returns an error when any required field is missing.
// It covers:
//   - valid vote creation
//   - missing BlockID
//   - missing SignerID
//   - missing SigData
func TestNewVote(t *testing.T) {
	const validView = uint64(1)

	t.Run("valid vote", func(t *testing.T) {
		blockID := unittest.IdentifierFixture()
		signerID := unittest.IdentifierFixture()
		sigData := []byte{0, 1, 2}

		uv := model.UntrustedVote{
			View:     validView,
			BlockID:  blockID,
			SignerID: signerID,
			SigData:  sigData,
		}

		v, err := model.NewVote(uv)
		assert.NoError(t, err)
		assert.NotNil(t, v)
		assert.Equal(t, validView, v.View)
		assert.Equal(t, blockID, v.BlockID)
		assert.Equal(t, signerID, v.SignerID)
		assert.Equal(t, sigData, v.SigData)
	})

	t.Run("empty BlockID", func(t *testing.T) {
		uv := model.UntrustedVote{
			View:     validView,
			BlockID:  flow.ZeroID,
			SignerID: unittest.IdentifierFixture(),
			SigData:  []byte{0, 1, 2},
		}

		v, err := model.NewVote(uv)
		assert.Error(t, err)
		assert.Nil(t, v)
		assert.Contains(t, err.Error(), "BlockID")
	})

	t.Run("empty SignerID", func(t *testing.T) {
		uv := model.UntrustedVote{
			View:     validView,
			BlockID:  unittest.IdentifierFixture(),
			SignerID: flow.ZeroID,
			SigData:  []byte{0, 1, 2},
		}

		v, err := model.NewVote(uv)
		assert.Error(t, err)
		assert.Nil(t, v)
		assert.Contains(t, err.Error(), "SignerID")
	})

	t.Run("empty SigData", func(t *testing.T) {
		uv := model.UntrustedVote{
			View:     validView,
			BlockID:  unittest.IdentifierFixture(),
			SignerID: unittest.IdentifierFixture(),
			SigData:  nil,
		}

		v, err := model.NewVote(uv)
		assert.Error(t, err)
		assert.Nil(t, v)
		assert.Contains(t, err.Error(), "SigData")
	})
}
