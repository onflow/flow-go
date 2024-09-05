package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/crypto"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	cadenceutils "github.com/onflow/flow-go/access/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
)

type Blocks interface {
	HeaderByID(id flow.Identifier) (*flow.Header, error)
	FinalizedHeader() (*flow.Header, error)
	SealedHeader() (*flow.Header, error)
}

type ProtocolStateBlocks struct {
	state protocol.State
}

func NewProtocolStateBlocks(state protocol.State) *ProtocolStateBlocks {
	return &ProtocolStateBlocks{state: state}
}

func (b *ProtocolStateBlocks) HeaderByID(id flow.Identifier) (*flow.Header, error) {
	header, err := b.state.AtBlockID(id).Head()
	if err != nil {
		if errors.Is(err, state.ErrUnknownSnapshotReference) {
			return nil, nil
		}

		return nil, err
	}

	return header, nil
}

func (b *ProtocolStateBlocks) FinalizedHeader() (*flow.Header, error) {
	return b.state.Final().Head()
}

func (b *ProtocolStateBlocks) SealedHeader() (*flow.Header, error) {
	return b.state.Sealed().Head()
}

// RateLimiter is an interface for checking if an address is rate limited.
// By convention, the address used is the payer field of a transaction.
// This rate limiter is applied when a transaction is first received by a
// node, meaning that if a transaction is rate-limited it will be dropped.
type RateLimiter interface {
	// IsRateLimited returns true if the address is rate limited
	IsRateLimited(address flow.Address) bool
}

type NoopLimiter struct{}

func NewNoopLimiter() *NoopLimiter {
	return &NoopLimiter{}
}

func (l *NoopLimiter) IsRateLimited(address flow.Address) bool {
	return false
}

type PayerBalanceMode int

const (
	Disabled PayerBalanceMode = iota
	WarnCheck
	EnforceCheck
)

func ParsePayerBalanceMode(s string) (PayerBalanceMode, error) {
	switch s {
	case Disabled.String():
		return Disabled, nil
	case WarnCheck.String():
		return WarnCheck, nil
	case EnforceCheck.String():
		return EnforceCheck, nil
	default:
		return 0, errors.New("invalid payer balance mode")
	}
}

func (m PayerBalanceMode) String() string {
	switch m {
	case Disabled:
		return "disabled"
	case WarnCheck:
		return "warn"
	case EnforceCheck:
		return "enforce"
	default:
		return ""
	}
}

type TransactionValidationOptions struct {
	Expiry                       uint
	ExpiryBuffer                 uint
	AllowEmptyReferenceBlockID   bool
	AllowUnknownReferenceBlockID bool
	MaxGasLimit                  uint64
	CheckScriptsParse            bool
	MaxTransactionByteSize       uint64
	MaxCollectionByteSize        uint64
	CheckPayerBalanceMode        PayerBalanceMode
}

type ValidationStep struct {
	check      func(*flow.TransactionBody) error
	failReason string
}

type TransactionValidator struct {
	blocks                       Blocks     // for looking up blocks to check transaction expiry
	chain                        flow.Chain // for checking validity of addresses
	options                      TransactionValidationOptions
	serviceAccountAddress        flow.Address
	limiter                      RateLimiter
	scriptExecutor               execution.ScriptExecutor
	verifyPayerBalanceScript     []byte
	transactionValidationMetrics module.TransactionValidationMetrics

	validationSteps []ValidationStep
}

func NewTransactionValidator(
	blocks Blocks,
	chain flow.Chain,
	transactionValidationMetrics module.TransactionValidationMetrics,
	options TransactionValidationOptions,
	executor execution.ScriptExecutor,
) (*TransactionValidator, error) {
	if options.CheckPayerBalanceMode != Disabled && executor == nil {
		return nil, errors.New("transaction validator cannot use checkPayerBalance with nil executor")
	}

	env := systemcontracts.SystemContractsForChain(chain.ChainID()).AsTemplateEnv()

	txValidator := &TransactionValidator{
		blocks:                       blocks,
		chain:                        chain,
		options:                      options,
		serviceAccountAddress:        chain.ServiceAddress(),
		limiter:                      NewNoopLimiter(),
		scriptExecutor:               executor,
		verifyPayerBalanceScript:     templates.GenerateVerifyPayerBalanceForTxExecution(env),
		transactionValidationMetrics: transactionValidationMetrics,
	}

	txValidator.initValidationSteps()

	return txValidator, nil
}

func NewTransactionValidatorWithLimiter(
	blocks Blocks,
	chain flow.Chain,
	options TransactionValidationOptions,
	transactionValidationMetrics module.TransactionValidationMetrics,
	rateLimiter RateLimiter,
) *TransactionValidator {
	txValidator := &TransactionValidator{
		blocks:                       blocks,
		chain:                        chain,
		options:                      options,
		serviceAccountAddress:        chain.ServiceAddress(),
		limiter:                      rateLimiter,
		transactionValidationMetrics: transactionValidationMetrics,
	}

	txValidator.initValidationSteps()

	return txValidator
}

func (v *TransactionValidator) initValidationSteps() {
	v.validationSteps = []ValidationStep{
		// rate limit transactions for specific payers.
		// a short term solution to prevent attacks that send too many failed transactions
		// if a transaction is from a payer that should be rate limited, all the following
		// checks will be skipped
		{v.checkRateLimitPayer, metrics.InvalidTransactionRateLimit},
		{v.checkTxSizeLimit, metrics.InvalidTransactionByteSize},
		{v.checkMissingFields, metrics.IncompleteTransaction},
		{v.checkGasLimit, metrics.InvalidGasLimit},
		{v.checkExpiry, metrics.ExpiredTransaction},
		{v.checkCanBeParsed, metrics.InvalidScript},
		{v.checkAddresses, metrics.InvalidAddresses},
		{v.checkSignatureFormat, metrics.InvalidSignature},
		{v.checkSignatureDuplications, metrics.DuplicatedSignature},
	}
}

func (v *TransactionValidator) Validate(ctx context.Context, tx *flow.TransactionBody) (err error) {

	for _, step := range v.validationSteps {
		if err = step.check(tx); err != nil {
			v.transactionValidationMetrics.TransactionValidationFailed(step.failReason)
			return err
		}
	}

	err = v.checkSufficientBalanceToPayForTransaction(ctx, tx)
	if err != nil {
		// we only return InsufficientBalanceError as it's a client-side issue
		// that requires action from a user. Other errors (e.g. parsing errors)
		// are 'internal' and related to script execution process. they shouldn't
		// prevent the transaction from proceeding.
		if IsInsufficientBalanceError(err) {
			v.transactionValidationMetrics.TransactionValidationFailed(metrics.InsufficientBalance)

			if v.options.CheckPayerBalanceMode == EnforceCheck {
				log.Warn().Err(err).Str("transactionID", tx.ID().String()).Str("payerAddress", tx.Payer.String()).Msg("enforce check error")
				return err
			}
		}

		// log and ignore all other errors
		v.transactionValidationMetrics.TransactionValidationSkipped()
		log.Info().Err(err).Msg("check payer validation skipped due to error")
	}

	// TODO replace checkSignatureFormat by verifying the account/payer signatures

	v.transactionValidationMetrics.TransactionValidated()

	return nil
}

func (v *TransactionValidator) checkRateLimitPayer(tx *flow.TransactionBody) error {
	if v.limiter.IsRateLimited(tx.Payer) {
		return InvalidTxRateLimitedError{
			Payer: tx.Payer,
		}
	}
	return nil
}

func (v *TransactionValidator) checkTxSizeLimit(tx *flow.TransactionBody) error {
	txSize := uint64(tx.ByteSize())
	// first check compatibility to collection byte size
	// this guarantees liveness
	if txSize >= v.options.MaxCollectionByteSize {
		return InvalidTxByteSizeError{
			Actual:  txSize,
			Maximum: v.options.MaxCollectionByteSize,
		}
	}
	// this logic need the reason we don't greenlist the service account against the collection size
	// limits is we can't verify the signature here yet.
	if tx.Payer == v.serviceAccountAddress {
		return nil
	}
	if txSize > v.options.MaxTransactionByteSize {
		return InvalidTxByteSizeError{
			Actual:  txSize,
			Maximum: v.options.MaxTransactionByteSize,
		}
	}
	return nil
}

func (v *TransactionValidator) checkMissingFields(tx *flow.TransactionBody) error {
	missingFields := tx.MissingFields()

	if v.options.AllowEmptyReferenceBlockID {
		missingFields = remove(missingFields, flow.TransactionFieldRefBlockID.String())
	}

	if len(missingFields) > 0 {
		return IncompleteTransactionError{MissingFields: missingFields}
	}

	return nil
}

func (v *TransactionValidator) checkGasLimit(tx *flow.TransactionBody) error {
	// if service account is the payer of the transaction accepts any gas limit
	// note that even though we don't enforce any limit here, exec node later
	// enforce a max value for any transaction
	if tx.Payer == v.serviceAccountAddress {
		return nil
	}
	if tx.GasLimit > v.options.MaxGasLimit || tx.GasLimit == 0 {
		return InvalidGasLimitError{
			Actual:  tx.GasLimit,
			Maximum: v.options.MaxGasLimit,
		}
	}

	return nil
}

// checkExpiry checks whether a transaction's reference block ID is
// valid. Returns nil if the reference is valid, returns an error if the
// reference is invalid or we failed to check it.
func (v *TransactionValidator) checkExpiry(tx *flow.TransactionBody) error {
	if tx.ReferenceBlockID == flow.ZeroID && v.options.AllowEmptyReferenceBlockID {
		return nil
	}

	// look up the reference block
	ref, err := v.blocks.HeaderByID(tx.ReferenceBlockID)
	if err != nil {
		return fmt.Errorf("could not get reference block: %w", err)
	}

	if ref == nil {
		// the transaction references an unknown block - at this point we decide
		// whether to consider it expired based on configuration
		if v.options.AllowUnknownReferenceBlockID {
			return nil
		}

		return ErrUnknownReferenceBlock
	}

	// get the latest finalized block we know about
	final, err := v.blocks.FinalizedHeader()
	if err != nil {
		return fmt.Errorf("could not get finalized header: %w", err)
	}

	diff := final.Height - ref.Height
	// check for overflow
	if ref.Height > final.Height {
		diff = 0
	}

	// discard transactions that are expired, or that will expire sooner than
	// our configured buffer allows
	if uint(diff) > v.options.Expiry-v.options.ExpiryBuffer {
		return ExpiredTransactionError{
			RefHeight:   ref.Height,
			FinalHeight: final.Height,
		}
	}

	return nil
}

func (v *TransactionValidator) checkCanBeParsed(tx *flow.TransactionBody) error {
	if v.options.CheckScriptsParse {
		_, err := parser.ParseProgram(nil, tx.Script, parser.Config{})
		if err != nil {
			return InvalidScriptError{ParserErr: err}
		}
	}

	return nil
}

func (v *TransactionValidator) checkAddresses(tx *flow.TransactionBody) error {
	for _, address := range append(tx.Authorizers, tx.Payer) {
		// we check whether this is a valid output of the address generator
		if !v.chain.IsValid(address) {
			return InvalidAddressError{Address: address}
		}
	}

	return nil
}

// every key (account, key index combination) can only be used once for signing
func (v *TransactionValidator) checkSignatureDuplications(tx *flow.TransactionBody) error {
	type uniqueKey struct {
		address flow.Address
		index   uint32
	}
	observedSigs := make(map[uniqueKey]bool)
	for _, sig := range append(tx.PayloadSignatures, tx.EnvelopeSignatures...) {
		if observedSigs[uniqueKey{sig.Address, sig.KeyIndex}] {
			return DuplicatedSignatureError{Address: sig.Address, KeyIndex: sig.KeyIndex}
		}
		observedSigs[uniqueKey{sig.Address, sig.KeyIndex}] = true
	}
	return nil
}

func (v *TransactionValidator) checkSignatureFormat(tx *flow.TransactionBody) error {
	for _, signature := range append(tx.PayloadSignatures, tx.EnvelopeSignatures...) {
		// check the format of the signature is valid.
		// a valid signature is an ECDSA signature of either P-256 or secp256k1 curve.
		ecdsaSignature := signature.Signature

		// check if the signature could be a P-256 signature
		valid, err := crypto.SignatureFormatCheck(crypto.ECDSAP256, ecdsaSignature)
		if err != nil {
			return fmt.Errorf("could not check the signature format (%s): %w", signature, err)
		}
		if valid {
			continue
		}

		// check if the signature could be a secp256k1 signature
		valid, err = crypto.SignatureFormatCheck(crypto.ECDSASecp256k1, ecdsaSignature)
		if err != nil {
			return fmt.Errorf("could not check the signature format (%s): %w", signature, err)
		}
		if valid {
			continue
		}

		return InvalidSignatureError{Signature: signature}
	}

	return nil
}

func (v *TransactionValidator) checkSufficientBalanceToPayForTransaction(ctx context.Context, tx *flow.TransactionBody) error {
	if v.options.CheckPayerBalanceMode == Disabled {
		return nil
	}

	header, err := v.blocks.SealedHeader()
	if err != nil {
		return fmt.Errorf("could not fetch block header: %w", err)
	}

	payerAddress := cadence.NewAddress(tx.Payer)
	inclusionEffort := cadence.UFix64(tx.InclusionEffort())
	gasLimit := cadence.UFix64(tx.GasLimit)

	args, err := cadenceutils.EncodeArgs([]cadence.Value{payerAddress, inclusionEffort, gasLimit})
	if err != nil {
		return fmt.Errorf("failed to encode cadence args for script executor: %w", err)
	}

	result, err := v.scriptExecutor.ExecuteAtBlockHeight(ctx, v.verifyPayerBalanceScript, args, header.Height)
	if err != nil {
		return fmt.Errorf("script finished with error: %w", err)
	}

	value, err := jsoncdc.Decode(nil, result)
	if err != nil {
		return fmt.Errorf("could not decode result value returned by script executor: %w", err)
	}

	canExecuteTransaction, requiredBalance, _, err := fvm.DecodeVerifyPayerBalanceResult(value)
	if err != nil {
		return fmt.Errorf("could not parse cadence value returned by script executor: %w", err)
	}

	// return no error if payer has sufficient balance
	if bool(canExecuteTransaction) {
		return nil
	}

	return InsufficientBalanceError{Payer: tx.Payer, RequiredBalance: requiredBalance}
}

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
