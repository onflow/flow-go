package validator_test

import (
	"context"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go/access/validator"
	validatormock "github.com/onflow/flow-go/access/validator/mock"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	execmock "github.com/onflow/flow-go/module/execution/mock"
	"github.com/onflow/flow-go/module/metrics"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestTransactionValidatorSuite(t *testing.T) {
	suite.Run(t, new(TransactionValidatorSuite))
}

type TransactionValidatorSuite struct {
	suite.Suite
	blocks           *validatormock.Blocks
	registersAsync   *execution.RegistersAsyncStore
	header           *flow.Header
	chain            flow.Chain
	validatorOptions validator.TransactionValidationOptions
	metrics          module.TransactionValidationMetrics
}

func (s *TransactionValidatorSuite) SetupTest() {
	s.metrics = metrics.NewNoopCollector()
	s.blocks = validatormock.NewBlocks(s.T())
	s.header = unittest.BlockHeaderFixture()

	registers := storagemock.NewRegisterSnapshotReader(s.T())
	s.registersAsync = execution.NewRegistersAsyncStore()
	require.NoError(s.T(), s.registersAsync.Initialize(registers))

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
	s.validatorOptions = validator.TransactionValidationOptions{
		CheckPayerBalanceMode:  validator.EnforceCheck,
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

	s.blocks.
		On("IndexedHeight").
		Return(s.header.Height, nil)

	scriptExecutor.
		On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, errors.New("script executor internal error")).
		Once()

	validator, err := validator.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor, s.registersAsync)
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

	s.blocks.
		On("IndexedHeight").
		Return(s.header.Height, nil)

	scriptExecutor.
		On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(actualResponse, nil).
		Once()

	validator, err := validator.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor, s.registersAsync)
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

	s.blocks.
		On("IndexedHeight").
		Return(s.header.Height, nil)

	scriptExecutor.
		On("ExecuteAtBlockHeight", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(actualResponse, nil).Twice()

	actualAccountResponse, err := unittest.AccountFixture()
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), actualAccountResponse)

	validateTx := func() error {
		txBody := unittest.TransactionBodyFixture()
		validator, err := validator.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor, s.registersAsync)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), validator)

		return validator.Validate(context.Background(), &txBody)
	}

	s.Run("with enforce check", func() {
		err := validateTx()

		expectedError := validator.InsufficientBalanceError{
			Payer:           unittest.AddressFixture(),
			RequiredBalance: requiredBalance,
		}
		assert.ErrorIs(s.T(), err, expectedError)
	})

	s.Run("with warn check", func() {
		s.validatorOptions.CheckPayerBalanceMode = validator.WarnCheck
		err := validateTx()
		assert.NoError(s.T(), err)
	})
}

func (s *TransactionValidatorSuite) TestTransactionValidator_SealedIndexedHeightThresholdLimit() {
	scriptExecutor := execmock.NewScriptExecutor(s.T())

	// setting indexed height to be behind of sealed by bigger number than allowed(DefaultSealedIndexedHeightThreshold)
	indexedHeight := s.header.Height - 40

	s.blocks.
		On("IndexedHeight").
		Return(indexedHeight, nil)

	validator, err := validator.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor, s.registersAsync)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), validator)

	txBody := unittest.TransactionBodyFixture()

	err = validator.Validate(context.Background(), &txBody)
	assert.NoError(s.T(), err)
}

func (s *TransactionValidatorSuite) TestTransactionValidator_SignatureValidation() {
	scriptExecutor := execmock.NewScriptExecutor(s.T())

	// setting indexed height to be behind of sealed by bigger number than allowed(DefaultSealedIndexedHeightThreshold)
	indexedHeight := s.header.Height - 40

	s.blocks.
		On("IndexedHeight").
		Return(indexedHeight, nil)

	validator, err := validator.NewTransactionValidator(s.blocks, s.chain, s.metrics, s.validatorOptions, scriptExecutor, s.registersAsync)
	assert.NoError(s.T(), err)
	assert.NotNil(s.T(), validator)

	// valid format signature
	ecdsaSignatureFixtureStr := "0f9c37c155fbb656d2049a8349a0fcd2dedfe27d1588c2f635f60df2052141c17f2e195790c55ce42c4f73b5cc7e194060fee641818e8d640e69f92d4777518d"
	ecdsaSignatureFixture, err := hex.DecodeString(ecdsaSignatureFixtureStr)
	require.NoError(s.T(), err)

	address := unittest.AddressFixture()
	transactionBody := flow.TransactionBody{
		Script: []byte("some script"),
		Arguments: [][]byte{
			[]byte("arg1"),
		},
		ReferenceBlockID: flow.HashToID([]byte("some block id")),
		GasLimit:         1000,
		Payer:            address,
		ProposalKey: flow.ProposalKey{
			Address:        address,
			KeyIndex:       0,
			SequenceNumber: 0,
		},
		Authorizers: []flow.Address{
			address,
		},
		PayloadSignatures: []flow.TransactionSignature{
			{
				Address:       address,
				KeyIndex:      0,
				Signature:     ecdsaSignatureFixture,
				SignerIndex:   0,
				ExtensionData: unittest.RandomBytes(3),
			},
		},
		EnvelopeSignatures: []flow.TransactionSignature{
			{
				Address:       address,
				KeyIndex:      1,
				Signature:     ecdsaSignatureFixture,
				SignerIndex:   0,
				ExtensionData: unittest.RandomBytes(3),
			},
		},
	}

	// detailed cases of signature validation are tested in model/flow/transaction_test.go
	cases := []struct {
		payloadSigExtensionData  []byte
		EnvelopeSigExtensionData []byte
		shouldError              bool
	}{
		{
			// happy path
			payloadSigExtensionData:  nil,
			EnvelopeSigExtensionData: nil,
			shouldError:              false,
		},
		{
			// invalid payload transaction
			payloadSigExtensionData:  []byte{10},
			EnvelopeSigExtensionData: nil,
			shouldError:              true,
		}, {
			// invalid envelope transaction
			payloadSigExtensionData:  nil,
			EnvelopeSigExtensionData: []byte{10},
			shouldError:              true,
		}, {
			// invalid envelope and payload transactions
			payloadSigExtensionData:  []byte{10},
			EnvelopeSigExtensionData: []byte{10},
			shouldError:              true,
		},
	}
	// test all cases
	for _, c := range cases {
		transactionBody.PayloadSignatures[0].ExtensionData = c.payloadSigExtensionData
		transactionBody.EnvelopeSignatures[0].ExtensionData = c.EnvelopeSigExtensionData
		err = validator.Validate(context.Background(), &transactionBody)
		if c.shouldError {
			assert.Error(s.T(), err)
		} else {
			assert.NoError(s.T(), err)
		}
	}
}
