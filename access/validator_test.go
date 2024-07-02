package access_test

import (
	"context"
	"github.com/onflow/flow-go/access"
	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"testing"
)

func TestTransactionValidator(t *testing.T) {
	blocks := accessmock.NewBlocks(t)
	assert.NotNil(t, blocks)

	header := unittest.BlockHeaderFixture()

	blocks.
		On("HeaderByID", mock.Anything).
		Return(header, nil).
		Once()

	blocks.
		On("FinalizedHeader", mock.Anything).
		Return(header, nil).
		Once()

	blocks.
		On("SealedHeader", mock.Anything).
		Return(header, nil).
		Once()

	chainID := flow.Testnet
	chain := chainID.Chain()

	options := access.TransactionValidationOptions{
		CheckPayerBalance:      true,
		MaxTransactionByteSize: 1000,
		MaxCollectionByteSize:  1000,
	}

	scriptExecutor := execmock.NewScriptExecutor(t)

	scriptExecutor.
		On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]byte("response data"), nil).
		Once()

	validator, err := access.NewTransactionValidator(blocks, chain, options, scriptExecutor)
	assert.NoError(t, err)
	assert.NotNil(t, validator)

	txBody := unittest.TransactionBodyFixture()

	err = validator.Validate(context.Background(), &txBody)
	assert.NoError(t, err)
}
