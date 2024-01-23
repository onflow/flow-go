package access

import (
	"errors"
	"fmt"

	"github.com/onflow/cadence/runtime/parser"
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state"
	"github.com/onflow/flow-go/state/protocol"
)

type Blocks interface {
	HeaderByID(id flow.Identifier) (*flow.Header, error)
	FinalizedHeader() (*flow.Header, error)
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

type TransactionValidationOptions struct {
	Expiry                       uint
	ExpiryBuffer                 uint
	AllowEmptyReferenceBlockID   bool
	AllowUnknownReferenceBlockID bool
	MaxGasLimit                  uint64
	CheckScriptsParse            bool
	MaxTransactionByteSize       uint64
	MaxCollectionByteSize        uint64
}

type TransactionValidator struct {
	blocks                Blocks     // for looking up blocks to check transaction expiry
	chain                 flow.Chain // for checking validity of addresses
	options               TransactionValidationOptions
	serviceAccountAddress flow.Address
	limiter               RateLimiter
}

func NewTransactionValidator(
	blocks Blocks,
	chain flow.Chain,
	options TransactionValidationOptions,
) *TransactionValidator {
	return &TransactionValidator{
		blocks:                blocks,
		chain:                 chain,
		options:               options,
		serviceAccountAddress: chain.ServiceAddress(),
		limiter:               NewNoopLimiter(),
	}
}

func NewTransactionValidatorWithLimiter(
	blocks Blocks,
	chain flow.Chain,
	options TransactionValidationOptions,
	rateLimiter RateLimiter,
) *TransactionValidator {
	return &TransactionValidator{
		blocks:                blocks,
		chain:                 chain,
		options:               options,
		serviceAccountAddress: chain.ServiceAddress(),
		limiter:               rateLimiter,
	}
}

func (v *TransactionValidator) Validate(tx *flow.TransactionBody) (err error) {
	// rate limit transactions for specific payers.
	// a short term solution to prevent attacks that send too many failed transactions
	// if a transaction is from a payer that should be rate limited, all the following
	// checks will be skipped
	err = v.checkRateLimitPayer(tx)
	if err != nil {
		return err
	}

	err = v.checkTxSizeLimit(tx)
	if err != nil {
		return err
	}

	err = v.checkMissingFields(tx)
	if err != nil {
		return err
	}

	err = v.checkGasLimit(tx)
	if err != nil {
		return err
	}

	err = v.checkExpiry(tx)
	if err != nil {
		return err
	}

	err = v.checkCanBeParsed(tx)
	if err != nil {
		return err
	}

	err = v.checkAddresses(tx)
	if err != nil {
		return err
	}

	err = v.checkSignatureFormat(tx)
	if err != nil {
		return err
	}

	err = v.checkSignatureDuplications(tx)
	if err != nil {
		return err
	}

	// TODO replace checkSignatureFormat by verifying the account/payer signatures

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
		index   uint64
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

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
