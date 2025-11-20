package convert_test

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/access/systemcollection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// TestConvertTransaction tests that converting a transaction to a protobuf message and back results in the
// same transaction body.
func TestConvertTransaction(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()
	tx := g.Transactions().Fixture()

	// add fields not included in the fixture
	arg, err := jsoncdc.Encode(cadence.NewAddress(g.Addresses().Fixture()))
	require.NoError(t, err)
	tx.Arguments = append(tx.Arguments, arg)

	msg := convert.TransactionToMessage(*tx)
	converted, err := convert.MessageToTransaction(msg, g.ChainID().Chain())
	require.NoError(t, err)

	assert.Equal(t, tx, &converted)
	assert.Equal(t, tx.ID(), converted.ID())
}

// TestConvertSystemTransaction tests that converting a system transaction to a protobuf message and
// back results in the same transaction body.
//
// System and scheduled transactions have nil/empty values for some fields. This test ensures that
// these fields are properly handled when converting to and from protobuf messages and the resulting
// transaction body identical.
func TestConvertSystemTransaction(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()
	events := g.PendingExecutionEvents().List(3)

	systemCollections, err := systemcollection.NewVersioned(g.ChainID().Chain(), systemcollection.Default(g.ChainID()))
	require.NoError(t, err)

	systemCollection, err := systemCollections.
		ByHeight(math.MaxUint64). // use the latest version
		SystemCollection(g.ChainID().Chain(), func() (flow.EventsList, error) {
			return events, nil
		})
	require.NoError(t, err)

	for _, tx := range systemCollection.Transactions {
		msg := convert.TransactionToMessage(*tx)
		converted, err := convert.MessageToTransaction(msg, g.ChainID().Chain())
		require.NoError(t, err)
		assert.Equal(t, tx, &converted)
		assert.Equal(t, tx.ID(), converted.ID())
	}
}
