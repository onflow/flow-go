package access_test

import (
	"context"
	"errors"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/runtime/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/access"
	accessmock "github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionValidatorSuite(t *testing.T) {
	suite.Run(t, new(TransactionValidatorSuite))
}

type TransactionValidatorSuite struct {
	suite.Suite
	blocks           *accessmock.Blocks
	header           *flow.Header
	chain            flow.Chain
	validatorOptions access.TransactionValidationOptions
	metrics          module.TransactionValidationMetrics
}

func (s *TransactionValidatorSuite) SetupTest() {
	s.metrics = metrics.NewNoopCollector()
	s.blocks = accessmock.NewBlocks(s.T())
	assert.NotNil(s.T(), s.blocks)

	s.header = unittest.BlockHeaderFixture()
	assert.NotNil(s.T(), s.header)

	s.blocks.
		On("HeaderByID", mock.Anything).
		Return(s.header, nil)

	s.blocks.
		On("FinalizedHeader").
		Return(s.header, nil)

	s.blocks.
		On("SealedHeader").
		Return(s.header, nil)

	s.chain = flow.Testnet.Chain()
	s.validatorOptions = access.TransactionValidationOptions{
		CheckPayerBalanceMode:  access.EnforceCheck,
		MaxTransactionByteSize: flow.DefaultMaxTransactionByteSize,
		MaxCollectionByteSize:  flow.DefaultMaxCollectionByteSize,
	}
}

var verifyPayerBalanceResultType = cadence.NewStructType(
	common.StringLocation("test"),
	"VerifyPayerBalanceResult",
	[]cadence.Field{
		{
			Identifier: fvm.VerifyPayerBalanceResultTypeCanExecuteTransactionFieldName,
			Type:       cadence.BoolType,
		},
		{
			Identifier: fvm.VerifyPayerBalanceResultTypeRequiredBalanceFieldName,
			Type:       cadence.UFix64Type,
		},
		{
			Identifier: fvm.VerifyPayerBalanceResultTypeMaximumTransactionFeesFieldName,
			Type:       cadence.UFix64Type,
		},
	},
	nil,
)

func (s *TransactionValidatorSuite) TestTransactionValidator_ScriptExecutorInternalError() {
	scriptExecutor := execmock.NewScriptExecutor(s.T())
	assert.NotNil(s.T(), scriptExecutor)

	scriptExecutor.
		On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("script executor internal error")).
		Once()

	validator, err := access.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), validator)

	txBody := unittest.TransactionBodyFixture()

	err = validator.Validate(context.Background(), &txBody)
	assert.NoError(s.T(), err)
}

func (s *TransactionValidatorSuite) TestTransactionValidator_SufficientBalance() {
	scriptExecutor := execmock.NewScriptExecutor(s.T())

	canExecuteTransaction := cadence.Bool(true)
	requiredBalance := cadence.UFix64(1000)
	maximumTransactionFees := cadence.UFix64(1000)
	fields := []cadence.Value{canExecuteTransaction, requiredBalance, maximumTransactionFees}

	actualResponseValue := cadence.NewStruct(fields).WithType(verifyPayerBalanceResultType)
	actualResponse, err := jsoncdc.Encode(actualResponseValue)
	assert.NoError(s.T(), err)

	scriptExecutor.
		On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(actualResponse, nil).
		Once()

	validator, err := access.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), validator)

	txBody := unittest.TransactionBodyFixture()

	err = validator.Validate(context.Background(), &txBody)
	assert.NoError(s.T(), err)
}

func (s *TransactionValidatorSuite) TestTransactionValidator_InsufficientBalance() {
	scriptExecutor := execmock.NewScriptExecutor(s.T())

	canExecuteTransaction := cadence.Bool(false)
	requiredBalance := cadence.UFix64(1000)
	maximumTransactionFees := cadence.UFix64(1000)
	fields := []cadence.Value{canExecuteTransaction, requiredBalance, maximumTransactionFees}

	actualResponseValue := cadence.NewStruct(fields).WithType(verifyPayerBalanceResultType)
	actualResponse, err := jsoncdc.Encode(actualResponseValue)
	assert.NoError(s.T(), err)

	scriptExecutor.
		On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(actualResponse, nil).Twice()

	actualAccountResponse, err := unittest.AccountFixture()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), actualAccountResponse)

	validateTx := func() error {
		txBody := unittest.TransactionBodyFixture()
		validator, err := access.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), validator)

		return validator.Validate(context.Background(), &txBody)
	}

	s.Run("with enforce check", func() {
		err := validateTx()

		expectedError := access.InsufficientBalanceError{
			Payer:           unittest.AddressFixture(),
			RequiredBalance: requiredBalance,
		}
		assert.ErrorIs(s.T(), err, expectedError)
	})

	s.Run("with warn check", func() {
		s.validatorOptions.CheckPayerBalanceMode = access.WarnCheck
		err := validateTx()
		assert.NoError(s.T(), err)
	})
}
