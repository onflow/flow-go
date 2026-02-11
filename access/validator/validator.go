package validator

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/cadence/parser"
	"github.com/onflow/crypto"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/access/ratelimit"
	cadenceutils "github.com/onflow/flow-go/access/utils"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
)

// DefaultSealedIndexedHeightThreshold is the default number of blocks between sealed and indexed height
// this sets a limit on how far into the past the payer validator will allow for checking the payer's balance.
const DefaultSealedIndexedHeightThreshold = 30

type Blocks interface {
	HeaderByID(id flow.Identifier) (*flow.Header, error)
	FinalizedHeader() (*flow.Header, error)
	SealedHeader() (*flow.Header, error)
	IndexedHeight() (uint64, error)
}

type ProtocolStateBlocks struct {
	state         protocol.State
	indexReporter state_synchronization.IndexReporter
}

func NewProtocolStateBlocks(state protocol.State, indexReporter state_synchronization.IndexReporter) *ProtocolStateBlocks {
	return &ProtocolStateBlocks{
		state:         state,
		indexReporter: indexReporter,
	}
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

// IndexedHeight returns the highest indexed height by calling corresponding function of indexReporter.
// Expected errors during normal operation:
// - access.IndexReporterNotInitialized - indexed reporter was not initialized.
func (b *ProtocolStateBlocks) IndexedHeight() (uint64, error) {
	if b.indexReporter != nil {
		return b.indexReporter.HighestIndexedHeight()
	}
	return 0, IndexReporterNotInitialized
}

// PayerBalanceMode represents the mode for checking the payer's balance
// when validating transactions. It controls whether and how the balance
// check is performed during transaction validation.
//
// There are few modes available:
//
//   - `Disabled` - Balance checking is completely disabled. No checks are
//     performed to verify if the payer has sufficient balance to cover the
//     transaction fees.
//   - `WarnCheck` - Balance is checked, and a warning is logged if the payer
//     does not have enough balance. The transaction is still accepted and
//     processed regardless of the check result.
//   - `EnforceCheck` - Balance is checked, and the transaction is rejected if
//     the payer does not have sufficient balance to cover the transaction fees.
type PayerBalanceMode int

const (
	// Disabled indicates that payer balance checking is turned off.
	Disabled PayerBalanceMode = iota

	// WarnCheck logs a warning if the payer's balance is insufficient, but does not prevent the transaction from being accepted.
	WarnCheck

	// EnforceCheck prevents the transaction from being accepted if the payer's balance is insufficient to cover transaction fees.
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

// TransactionValidator implements transaction validation logic for Access and Collection Nodes.
// NOTE: This validation logic is a simplified interim approach: Collection/Access Nodes cannot reliably validate transaction signatures or payer balance.
// The long-term design for extending validation to cover these cases is described in the Sweet Onion Plan
// (https://flowfoundation.notion.site/Sweet-Onion-Plan-eae4db664feb459598879b49ccf2aa85).
type TransactionValidator struct {
	blocks                       Blocks     // for looking up blocks to check transaction expiry
	chain                        flow.Chain // for checking validity of addresses
	options                      TransactionValidationOptions
	serviceAccountAddress        flow.Address
	limiter                      ratelimit.RateLimiter
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
		limiter:                      ratelimit.NewNoopLimiter(),
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
	rateLimiter ratelimit.RateLimiter,
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
		{v.checkAccounts, metrics.DuplicatedSignature},
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

func (v *TransactionValidator) checkCanBeParsed(tx *flow.TransactionBody) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if panicErr, ok := r.(error); ok {
				err = InvalidScriptError{ParserErr: panicErr}
			} else {
				err = InvalidScriptError{ParserErr: fmt.Errorf("non-error-typed panic: %v", r)}
			}
		}
	}()
	if v.options.CheckScriptsParse {
		_, parseErr := parser.ParseProgram(nil, tx.Script, parser.Config{})
		if parseErr != nil {
			return InvalidScriptError{ParserErr: parseErr}
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

// Checks related to the accounts used in the transaction:
//   - no duplicate account keys signing
//   - no unrelated account signatures (from accounts that are neither proposer, payer, nor authorizers)
//   - at least one payer envelope signature
//   - at least one proposer signature
//   - at least one signature per authorizer
func (v *TransactionValidator) checkAccounts(tx *flow.TransactionBody) error {
	// check for duplicate account key
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
	// check for minimum account signatures
	observedEnvelopeSig := make(map[flow.Address]bool)
	observedPayloadSig := make(map[flow.Address]bool)
	for _, sig := range tx.EnvelopeSignatures {
		observedEnvelopeSig[sig.Address] = true
	}
	for _, sig := range tx.PayloadSignatures {
		observedPayloadSig[sig.Address] = true
	}

	if !observedEnvelopeSig[tx.Payer] {
		return MissingSignatureError{Address: tx.Payer, Message: "payer envelope signature is missing"}
	}

	if !observedEnvelopeSig[tx.ProposalKey.Address] && !observedPayloadSig[tx.ProposalKey.Address] {
		return MissingSignatureError{Address: tx.ProposalKey.Address, Message: "proposer signature on either payload or envelope is missing"}
	}

	for _, authorizer := range tx.Authorizers {
		if authorizer == tx.Payer || authorizer == tx.ProposalKey.Address {
			// at this point, payer and proposer are guaranteed to have signatures
			continue
		}
		if !observedEnvelopeSig[authorizer] && !observedPayloadSig[authorizer] {
			return MissingSignatureError{Address: authorizer, Message: "authorizer signature on either payload or envelope is missing"}
		}
	}

	// check for unrelated account signatures
	relatedAccounts := make(map[flow.Address]struct{})
	relatedAccounts[tx.Payer] = struct{}{}
	relatedAccounts[tx.ProposalKey.Address] = struct{}{}
	for _, authorizer := range tx.Authorizers {
		relatedAccounts[authorizer] = struct{}{}
	}
	for _, sig := range append(tx.PayloadSignatures, tx.EnvelopeSignatures...) {
		if _, ok := relatedAccounts[sig.Address]; !ok {
			return UnrelatedAccountSignatureError{Address: sig.Address}
		}
	}
	return nil
}

// checkSignatureFormat checks the format of each transaction signature independently.
// The current checks are:
//   - the format is correct with regards to the authentication scheme
//   - sanity check that the cryptographic signature can be an ECDSA signature of either P-256 or secp256k1 curve
//
// TODO replace checkSignatureFormat with verifying the account/payer signatures when
// collection nodes can access the account public keys.
func (v *TransactionValidator) checkSignatureFormat(tx *flow.TransactionBody) error {
	for _, signature := range tx.PayloadSignatures {
		valid, _ := signature.ValidateExtensionDataAndReconstructMessage(tx.PayloadMessage())
		if !valid {
			return InvalidAuthenticationSchemeFormatError{Signature: signature}
		}
	}

	for _, signature := range tx.EnvelopeSignatures {
		valid, _ := signature.ValidateExtensionDataAndReconstructMessage(tx.EnvelopeMessage())
		if !valid {
			return InvalidAuthenticationSchemeFormatError{Signature: signature}
		}
	}

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

		return InvalidRawSignatureError{Signature: signature}
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

	indexedHeight, err := v.blocks.IndexedHeight()
	if err != nil {
		return fmt.Errorf("could not get indexed height: %w", err)
	}

	// we use latest indexed block to get the most up-to-date state data available for executing scripts.
	// check here to make sure indexing is within an acceptable tolerance of sealing to avoid issues
	// if indexing falls behind
	sealedHeight := header.Height
	if indexedHeight < sealedHeight-DefaultSealedIndexedHeightThreshold {
		return IndexedHeightFarBehindError{SealedHeight: sealedHeight, IndexedHeight: indexedHeight}
	}

	payerAddress := cadence.NewAddress(tx.Payer)
	inclusionEffort := cadence.UFix64(tx.InclusionEffort())
	gasLimit := cadence.UFix64(tx.GasLimit)

	args, err := cadenceutils.EncodeArgs([]cadence.Value{payerAddress, inclusionEffort, gasLimit})
	if err != nil {
		return fmt.Errorf("failed to encode cadence args for script executor: %w", err)
	}

	result, err := v.scriptExecutor.ExecuteAtBlockHeight(ctx, v.verifyPayerBalanceScript, args, indexedHeight)
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
	if canExecuteTransaction {
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
