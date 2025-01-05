package parser

import (
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

type TransactionSignature flow.TransactionSignature

func (t *TransactionSignature) Parse(raw interface{}) error {
	sigMap, ok := raw.(map[string]interface{})
	if !ok {
		return fmt.Errorf("'signature' must be a JSON object")
	}

	addressStr, ok := sigMap["address"].(string)
	if !ok {
		return fmt.Errorf("'address' must be a string")
	}
	address, err := flow.StringToAddress(addressStr)
	if err != nil {
		return fmt.Errorf("invalid 'address': %w", err)
	}

	signerIndexStr, ok := sigMap["signer_index"].(string) // JSON numbers are unmarshalled as float64
	if !ok {
		return fmt.Errorf("'signer_index' must be a string")
	}
	signerIndex, err := util.ToInt(signerIndexStr)
	if err != nil {
		return fmt.Errorf("invalid 'signer_index': %w", err)
	}

	keyIndexStr, ok := sigMap["key_index"].(string)
	if !ok {
		return fmt.Errorf("'key_index' must be a string")
	}
	keyIndex, err := util.ToUint32(keyIndexStr)
	if err != nil {
		return fmt.Errorf("invalid 'key_index': %w", err)
	}

	signatureStr, ok := sigMap["signature"].(string)
	if !ok {
		return fmt.Errorf("'signature' must be a string")
	}
	signature, err := util.FromBase64(signatureStr)
	if err != nil {
		return fmt.Errorf("invalid 'signature': %w", err)
	}

	*t = TransactionSignature{
		Address:     address,
		SignerIndex: signerIndex,
		KeyIndex:    keyIndex,
		Signature:   signature,
	}

	return nil
}

func (t TransactionSignature) Flow() flow.TransactionSignature {
	return flow.TransactionSignature(t)
}

type TransactionSignatures []TransactionSignature

func (t *TransactionSignatures) Parse(raw []interface{}) error {
	// make a map to have only unique values as keys
	sigs := make(TransactionSignatures, 0)
	uniqueSigs := make(map[string]bool) //TODO: check it we need it
	for _, r := range raw {
		var sig TransactionSignature
		err := sig.Parse(r)
		if err != nil {
			return err
		}

		if !uniqueSigs[sig.Flow().String()] {
			uniqueSigs[sig.Flow().String()] = true
			sigs = append(sigs, sig)
		}
	}

	*t = sigs
	return nil
}

func (t TransactionSignatures) Flow() []flow.TransactionSignature {
	sigs := make([]flow.TransactionSignature, len(t))
	for j, sig := range t {
		sigs[j] = sig.Flow()
	}
	return sigs
}
