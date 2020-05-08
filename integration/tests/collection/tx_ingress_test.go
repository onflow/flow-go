package collection

import (
	"context"
	"testing"

	sdk "github.com/onflow/flow-go-sdk"
	sdkclient "github.com/onflow/flow-go-sdk/client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// Test sending various invalid transactions to a single-cluster configuration.
// The transactions should be rejected by the collection node and not included
// in any collection.
func (suite *CollectorSuite) TestTransactionIngress_InvalidTransaction() {
	t := suite.T()

	suite.SetupTest("col_txingress_invalid", 3, 1)

	// pick a collector to test against
	col1 := suite.Collector(0, 0)

	client, err := sdkclient.New(col1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
	require.Nil(t, err)

	t.Run("missing reference block id", func(t *testing.T) {
		malformed := suite.NextTransaction(func(tx *sdk.Transaction) {
			tx.SetReferenceBlockID(sdk.ZeroID)
		})

		expected := ingest.IncompleteTransactionError{
			Missing: []string{flow.TransactionFieldRefBlockID.String()},
		}

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("missing script", func(t *testing.T) {
		malformed := suite.NextTransaction(func(tx *sdk.Transaction) {
			tx.SetScript(nil)
		})

		expected := ingest.IncompleteTransactionError{
			Missing: []string{flow.TransactionFieldScript.String()},
		}

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})
	t.Run("expired transaction", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("non-existent reference block ID", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("unparseable script", func(t *testing.T) {
		// TODO script parsing not implemented
		t.Skip()
	})
	t.Run("invalid signature", func(t *testing.T) {
		// TODO signature validation not implemented
		t.Skip()
	})
	t.Run("invalid sequence number", func(t *testing.T) {
		// TODO nonce validation not implemented
		t.Skip()
	})
	t.Run("insufficient payer balance", func(t *testing.T) {
		// TODO balance checking not implemented
		t.Skip()
	})
}
