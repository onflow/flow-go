package access

import (
	"fmt"

	"github.com/onflow/cadence/runtime/parser"

	"github.com/dapperlabs/flow-go/model/flow"
)

type BlockHeaderByID func(id flow.Identifier) (*flow.Header, error)
type FinalizedBlockHeader func() (*flow.Header, error)

type TransactionValidator struct {
	blockHeaderByID      BlockHeaderByID
	finalizedBlockHeader FinalizedBlockHeader
	options              TransactionValidationOptions
}

type TransactionValidationOptions struct {
	Expiry                     uint
	ExpiryBuffer               uint
	AllowUnknownReferenceBlock bool
	MaxGasLimit                uint64
	CheckScriptsParse          bool
}

func NewTransactionValidator(
	blockHeaderByID BlockHeaderByID,
	finalizedBlockHeader FinalizedBlockHeader,
	options TransactionValidationOptions,
) *TransactionValidator {
	return &TransactionValidator{
		blockHeaderByID:      blockHeaderByID,
		finalizedBlockHeader: finalizedBlockHeader,
		options:              options,
	}
}

func (v *TransactionValidator) Validate(tx *flow.TransactionBody) (err error) {
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

func (v *TransactionValidator) checkMissingFields(tx *flow.TransactionBody) error {
	missingFields := tx.MissingFields()
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

	// look up the reference block
	ref, err := v.blockHeaderByID(tx.ReferenceBlockID)
	if err != nil {
		return fmt.Errorf("could not get reference block: %w", err)
	}

	if ref == nil {
		// the transaction references an unknown block - at this point we decide
		// whether to consider it expired based on configuration
		if v.options.AllowUnknownReferenceBlock {
			return nil
		}

		return ErrUnknownReferenceBlock
	}

	// get the latest finalized block we know about
	final, err := v.finalizedBlockHeader()
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
	if uint(diff) > v.options.Expiry-v.options.Expiry {
		return ExpiredTransactionError{
			RefHeight:   ref.Height,
			FinalHeight: final.Height,
		}
	}

	return nil
}

func (v *TransactionValidator) checkCanBeParsed(tx *flow.TransactionBody) error {
	if v.options.CheckScriptsParse {
		_, _, err := parser.ParseProgram(string(tx.Script))
		if err != nil {
			return InvalidScriptError{ParserErr: err}
		}
	}

	return nil
}
