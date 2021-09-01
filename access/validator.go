package access

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/crypto"

	"github.com/onflow/cadence/runtime/parser2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
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
		if errors.Is(err, storage.ErrNotFound) {
			return nil, nil
		}

		return nil, err
	}

	return header, nil
}

func (b *ProtocolStateBlocks) FinalizedHeader() (*flow.Header, error) {
	return b.state.Final().Head()
}

type TransactionValidationOptions struct {
	Expiry                       uint
	ExpiryBuffer                 uint
	AllowEmptyReferenceBlockID   bool
	AllowUnknownReferenceBlockID bool
	MaxGasLimit                  uint64
	CheckScriptsParse            bool
	// MaxAddressIndex is a simple spam prevention measure. It rejects any
	// transactions referencing an address with index newer than the specified
	// maximum. A zero value indicates no address checking.
	MaxAddressIndex        uint64
	MaxTransactionByteSize uint64
	MaxCollectionByteSize  uint64
}

type TransactionValidator struct {
	blocks                Blocks     // for looking up blocks to check transaction expiry
	chain                 flow.Chain // for checking validity of addresses
	options               TransactionValidationOptions
	serviceAccountAddress flow.Address
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
	}
}

func (v *TransactionValidator) Validate(tx *flow.TransactionBody) (err error) {
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
		_, err := parser2.ParseProgram(string(tx.Script))
		if err != nil {
			return InvalidScriptError{ParserErr: err}
		}
	}

	return nil
}

func (v *TransactionValidator) checkAddresses(tx *flow.TransactionBody) error {

	for _, address := range append(tx.Authorizers, tx.Payer) {
		// first we check objective validity, essentially whether or not this
		// is a valid output of the address generator
		if !v.chain.IsValid(address) {
			return InvalidAddressError{Address: address}
		}

		// skip second check if not configured
		if v.options.MaxAddressIndex == 0 {
			continue
		}

		// next we check subjective validity based on the configured maximum index
		index, err := v.chain.IndexFromAddress(address)
		if err != nil {
			return fmt.Errorf("could not get index for address (%s): %w", address, err)
		}
		if index > v.options.MaxAddressIndex {
			return InvalidAddressError{Address: address}
		}
	}

	return nil
}

// every key (account, key index combination) can only be used once for signing
func (v *TransactionValidator) checkSignatureDuplications(tx *flow.TransactionBody) error {
	observedSigs := make(map[string]bool)
	for _, sig := range append(tx.PayloadSignatures, tx.EnvelopeSignatures...) {
		keyStr := sig.UniqueKeyString()
		if observedSigs[keyStr] {
			return DuplicatedSignatureError{Address: sig.Address, KeyIndex: sig.KeyIndex}
		}
		observedSigs[keyStr] = true
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
