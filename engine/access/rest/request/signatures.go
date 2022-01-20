package request

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionSignature flow.TransactionSignature

func (s *TransactionSignature) Parse(
	rawAddress string,
	rawSignerIndex string,
	rawKeyIndex string,
	rawSignature string,
) error {
	var address Address
	err := address.Parse(rawAddress)
	if err != nil {
		return err
	}

	sigIndex, err := util.ToUint64(rawSignerIndex)
	if err != nil {
		return fmt.Errorf("invalid signer index: %w", err)
	}

	keyIndex, err := util.ToUint64(rawKeyIndex)
	if err != nil {
		return fmt.Errorf("invalid key index: %w", err)
	}

	var signature Signature
	err = signature.Parse(rawSignature)
	if err != nil {
		return fmt.Errorf("invalid signature: %w", err)
	}

	*s = TransactionSignature(flow.TransactionSignature{
		Address:     address.Flow(),
		SignerIndex: int(sigIndex),
		KeyIndex:    keyIndex,
		Signature:   signature,
	})

	return nil
}

func (s TransactionSignature) Flow() flow.TransactionSignature {
	return flow.TransactionSignature(s)
}

type TransactionSignatures []TransactionSignature

func (t *TransactionSignatures) Parse(rawSigs []models.TransactionSignature) error {
	signatures := make([]TransactionSignature, len(rawSigs))
	for i, sig := range rawSigs {
		var signature TransactionSignature
		err := signature.Parse(sig.Address, sig.SignerIndex, sig.KeyIndex, sig.Signature)
		if err != nil {
			return err
		}
		signatures[i] = signature
	}

	*t = signatures
	return nil
}

func (t TransactionSignatures) Flow() []flow.TransactionSignature {
	sigs := make([]flow.TransactionSignature, len(t))
	for i, sig := range t {
		sigs[i] = sig.Flow()
	}
	return sigs
}

type Signature []byte

func (s *Signature) Parse(raw string) error {
	if raw == "" {
		return fmt.Errorf("missing value")
	}

	signatureBytes, err := util.FromBase64(raw)
	if err != nil {
		return fmt.Errorf("invalid encoding")
	}

	*s = signatureBytes
	return nil
}

func (s Signature) Flow() []byte {
	return s
}
