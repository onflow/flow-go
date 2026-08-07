package inspection_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/fvm/inspection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestAuthorizingSigners verifies AuthorizingSigners against its documented
// contract.
func TestAuthorizingSigners(t *testing.T) {
	proposer := unittest.RandomAddressFixture()
	payer := unittest.RandomAddressFixture()
	authorizer1 := unittest.RandomAddressFixture()
	authorizer2 := unittest.RandomAddressFixture()

	t.Run("ordering: proposer, then authorizers in insertion order", func(t *testing.T) {
		tb := flow.TransactionBody{
			ProposalKey: flow.ProposalKey{Address: proposer},
			Payer:       payer,
			Authorizers: []flow.Address{authorizer1, authorizer2},
		}

		assert.Equal(t,
			[]flow.Address{proposer, authorizer1, authorizer2},
			inspection.AuthorizingSigners(&tb),
		)
	})

	t.Run("payer is excluded even when it is the only signer", func(t *testing.T) {
		tb := flow.TransactionBody{
			Payer: payer,
		}

		assert.Empty(t, inspection.AuthorizingSigners(&tb))
	})

	t.Run("deduplication: account in multiple roles appears once at first occurrence", func(t *testing.T) {
		// proposer is also an authorizer, and authorizer1 is repeated.
		tb := flow.TransactionBody{
			ProposalKey: flow.ProposalKey{Address: proposer},
			Payer:       payer,
			Authorizers: []flow.Address{authorizer1, proposer, authorizer1, authorizer2},
		}

		assert.Equal(t,
			[]flow.Address{proposer, authorizer1, authorizer2},
			inspection.AuthorizingSigners(&tb),
		)
	})

	t.Run("empty addresses are omitted", func(t *testing.T) {
		// No proposer set (e.g. the system transaction).
		tb := flow.TransactionBody{
			Authorizers: []flow.Address{authorizer1},
		}

		assert.Equal(t, []flow.Address{authorizer1}, inspection.AuthorizingSigners(&tb))
	})

	t.Run("no signers returns empty", func(t *testing.T) {
		tb := flow.TransactionBody{}
		assert.Empty(t, inspection.AuthorizingSigners(&tb))
	})

	t.Run("nil transaction body returns nil", func(t *testing.T) {
		assert.Nil(t, inspection.AuthorizingSigners(nil))
	})
}
