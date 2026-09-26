package validator_test

import (
	"context"
	"testing"

	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

// fixedBlocks is a minimal Blocks implementation for fuzz testing that always
// returns the same fixed header, keeping the harness deterministic and offline.
type fixedBlocks struct {
	header *flow.Header
}

func (b *fixedBlocks) HeaderByID(_ flow.Identifier) (*flow.Header, error) {
	return b.header, nil
}

func (b *fixedBlocks) FinalizedHeader() (*flow.Header, error) {
	return b.header, nil
}

func (b *fixedBlocks) SealedHeader() (*flow.Header, error) {
	return b.header, nil
}

func (b *fixedBlocks) IndexedHeight() (uint64, error) {
	return b.header.Height, nil
}

func FuzzTransactionValidatorValidate(f *testing.F) {
	header := unittest.BlockHeaderFixture()
	blocks := &fixedBlocks{header: header}
	chain := flow.Testnet.Chain()
	opts := validator.TransactionValidationOptions{
		Expiry:                       flow.DefaultTransactionExpiry,
		ExpiryBuffer:                 0,
		AllowEmptyReferenceBlockID:   false,
		AllowUnknownReferenceBlockID: true,
		MaxGasLimit:                  flow.DefaultMaxTransactionGasLimit,
		CheckScriptsParse:            false,
		MaxTransactionByteSize:       flow.DefaultMaxTransactionByteSize,
		MaxCollectionByteSize:        flow.DefaultMaxCollectionByteSize,
		CheckPayerBalanceMode:        validator.Disabled,
	}

	v, err := validator.NewTransactionValidator(blocks, chain, metrics.NewNoopCollector(), opts, nil)
	if err != nil {
		f.Fatalf("failed to build validator: %v", err)
	}

	// seed: valid transaction from fixture
	seed := unittest.TransactionBodyFixture()
	f.Add(seed.Script, seed.GasLimit, seed.Payer.Bytes())

	// seed: minimal empty script
	f.Add([]byte("access(all) fun main() {}"), uint64(10), unittest.AddressFixture().Bytes())
	f.Add([]byte{}, uint64(0), []byte{})

	f.Fuzz(func(t *testing.T, script []byte, gasLimit uint64, payerBytes []byte) {
		tx := unittest.TransactionBodyFixture()
		tx.Script = script
		tx.GasLimit = gasLimit
		if len(payerBytes) >= flow.AddressLength {
			copy(tx.Payer[:], payerBytes[:flow.AddressLength])
		}

		err1 := v.Validate(context.Background(), &tx)
		err2 := v.Validate(context.Background(), &tx)

		// accept/reject decision must be deterministic across two calls
		if (err1 == nil) != (err2 == nil) {
			t.Fatalf("Validate returned non-deterministic result: first=%v second=%v", err1, err2)
		}
	})
}
