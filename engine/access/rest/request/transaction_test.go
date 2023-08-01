package request

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func buildTransaction() map[string]interface{} {
	tx := unittest.TransactionFixture()
	tx.Arguments = [][]uint8{}
	tx.PayloadSignatures = []flow.TransactionSignature{}
	auth := make([]string, len(tx.Authorizers))
	for i, a := range tx.Authorizers {
		auth[i] = a.String()
	}

	return map[string]interface{}{
		"script":             util.ToBase64(tx.Script),
		"arguments":          tx.Arguments,
		"reference_block_id": tx.ReferenceBlockID.String(),
		"gas_limit":          fmt.Sprintf("%d", tx.GasLimit),
		"payer":              tx.Payer.String(),
		"proposal_key": map[string]interface{}{
			"address":         tx.ProposalKey.Address.String(),
			"key_index":       fmt.Sprintf("%d", tx.ProposalKey.KeyIndex),
			"sequence_number": fmt.Sprintf("%d", tx.ProposalKey.SequenceNumber),
		},
		"authorizers": auth,
		"envelope_signatures": []map[string]interface{}{{
			"address":   tx.EnvelopeSignatures[0].Address.String(),
			"key_index": fmt.Sprintf("%d", tx.EnvelopeSignatures[0].KeyIndex),
			"signature": util.ToBase64(tx.EnvelopeSignatures[0].Signature),
		}},
	}
}

func transactionToReader(tx map[string]interface{}) io.Reader {
	res, _ := json.Marshal(tx)
	return bytes.NewReader(res)
}

func TestTransaction_InvalidParse(t *testing.T) {
	tests := []struct {
		inputField string
		inputValue string
		output     string
	}{
		{"script", "-1", "invalid transaction script encoding"},
		{"arguments", "-1", `request body contains an invalid value for the "arguments" field (at position 17)`},
		{"reference_block_id", "-1", "invalid reference block ID: invalid ID format"},
		{"gas_limit", "-1", "invalid gas limit: value must be an unsigned 64 bit integer"},
		{"payer", "-1", "invalid payer: invalid address"},
		{"authorizers", "-1", `request body contains an invalid value for the "authorizers" field (at position 34)`},
		{"proposal_key", "-1", `request body contains an invalid value for the "proposal_key" field (at position 288)`},
		{"envelope_signatures", "", `request body contains an invalid value for the "envelope_signatures" field (at position 75)`},
		{"envelope_signatures", "[]", `request body contains an invalid value for the "envelope_signatures" field (at position 77)`},
	}

	for _, test := range tests {
		tx := buildTransaction()
		tx[test.inputField] = test.inputValue
		input := transactionToReader(tx)

		var transaction Transaction
		err := transaction.Parse(input, flow.Testnet.Chain())

		assert.EqualError(t, err, test.output)
	}

	keyTests := []struct {
		inputField string
		inputValue string
		output     string
	}{
		{"address", "-1", "invalid address"},
		{"key_index", "-1", `invalid key index: value must be an unsigned 64 bit integer`},
		{"sequence_number", "-1", "invalid sequence number: value must be an unsigned 64 bit integer"},
	}

	for _, test := range keyTests {
		tx := buildTransaction()
		tx["proposal_key"].(map[string]interface{})[test.inputField] = test.inputValue
		input := transactionToReader(tx)

		var transaction Transaction
		err := transaction.Parse(input, flow.Testnet.Chain())

		assert.EqualError(t, err, test.output)
	}

	sigTests := []struct {
		inputField string
		inputValue string
		output     string
	}{
		{"address", "-1", "invalid address"},
		{"key_index", "-1", `invalid key index: value must be an unsigned 64 bit integer`},
		{"signature", "-1", "invalid signature: invalid encoding"},
	}

	for _, test := range sigTests {
		tx := buildTransaction()
		tx["envelope_signatures"].([]map[string]interface{})[0][test.inputField] = test.inputValue
		input := transactionToReader(tx)

		var transaction Transaction
		err := transaction.Parse(input, flow.Testnet.Chain())

		assert.EqualError(t, err, test.output)
	}
}

func TestTransaction_ValidParse(t *testing.T) {
	script := `access(all) fun main() {}`
	tx := buildTransaction()
	tx["script"] = util.ToBase64([]byte(script))
	input := transactionToReader(tx)

	var transaction Transaction
	err := transaction.Parse(input, flow.Testnet.Chain())

	assert.NoError(t, err)
	assert.Equal(t, tx["payer"], transaction.Flow().Payer.String())
	assert.Equal(t, script, string(transaction.Flow().Script))
	assert.Equal(t, tx["reference_block_id"], transaction.Flow().ReferenceBlockID.String())
	assert.Equal(t, tx["gas_limit"], fmt.Sprint(transaction.Flow().GasLimit))
	assert.Equal(t, len(tx["authorizers"].([]string)), len(transaction.Flow().Authorizers))
}
