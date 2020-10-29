package access

import (
	"errors"
	"fmt"

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
	MaxTxSizeLimit               uint64
}

type TransactionValidator struct {
	blocks  Blocks
	options TransactionValidationOptions
}

func NewTransactionValidator(
	blocks Blocks,
	options TransactionValidationOptions,
) *TransactionValidator {
	return &TransactionValidator{
		blocks:  blocks,
		options: options,
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

	// TODO check account/payer signatures

	return nil
}

func (v *TransactionValidator) checkTxSizeLimit(tx *flow.TransactionBody) error {
	txSize := uint64(tx.ByteSize())
	if txSize > v.options.MaxTxSizeLimit {
		return InvalidTxByteSizeError{
			Actual:  txSize,
			Maximum: v.options.MaxTxSizeLimit,
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
	if tx.GasLimit > v.options.MaxGasLimit {
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

func remove(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}
