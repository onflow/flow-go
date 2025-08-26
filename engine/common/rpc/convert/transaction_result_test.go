package convert_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertTransactionResult(t *testing.T) {
	t.Parallel()

	expected := txResultFixture()

	msg := convert.TransactionResultToMessage(expected)
	converted, err := convert.MessageToTransactionResult(msg)
	require.NoError(t, err)

	require.Equal(t, expected, converted)
}

func TestConvertTransactionResults(t *testing.T) {
	t.Parallel()

	expected := []*accessmodel.TransactionResult{
		txResultFixture(),
		txResultFixture(),
	}

	msg := convert.TransactionResultsToMessage(expected)
	converted, err := convert.MessageToTransactionResults(msg)
	require.NoError(t, err)

	require.Equal(t, expected, converted)
}

func txResultFixture() *accessmodel.TransactionResult {
	return &accessmodel.TransactionResult{
		Status:        flow.TransactionStatusExecuted,
		StatusCode:    0,
		Events:        unittest.EventsFixture(3),
		ErrorMessage:  "",
		BlockID:       unittest.IdentifierFixture(),
		TransactionID: unittest.IdentifierFixture(),
		CollectionID:  unittest.IdentifierFixture(),
		BlockHeight:   100,
	}
}
