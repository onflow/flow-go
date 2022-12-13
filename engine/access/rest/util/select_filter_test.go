package util_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/engine/access/rest/util"

	"github.com/stretchr/testify/require"
)

func TestSelectFilter(t *testing.T) {

	testVectors := []struct {
		input       string
		output      string
		keys        []string
		description string
	}{
		{
			input:       `{ "a": 1, "b": 2}`,
			output:      `{ "b": 2}`,
			keys:        []string{"b"},
			description: "single object without nested fields",
		},
		{
			input:       `[{ "a": 1, "b": 2}]`,
			output:      `[{ "b": 2}]`,
			keys:        []string{"b"},
			description: "array of objects without nested fields",
		},
		{
			input:       `{ "a": 1, "b": {"c":2, "d":3}}`,
			output:      `{ "b": {"c": 2}}`,
			keys:        []string{"b.c"},
			description: "single object with nested fields",
		},
		{
			input:       `[{ "a": 1, "b": {"c":2, "d":3}}]`,
			output:      `[{ "b": {"c": 2}}]`,
			keys:        []string{"b.c"},
			description: "array of objects with nested fields",
		},
		{
			input:       `{ "a": 1, "b": [{"c":2}, {"c": 3}]}`,
			output:      `{ "b": [{"c":2}, {"c": 3}]}`,
			keys:        []string{"b.c"},
			description: "single object with arrays as values",
		},
	}

	for _, tv := range testVectors {
		testFilter(t, tv.input, tv.output, tv.description, tv.keys...)
	}

}

func testFilter(t *testing.T, inputJson, exepectedJson string, description string, selectKeys ...string) {
	var outputInterface interface{}
	if strings.HasPrefix(inputJson, "{") {
		outputInterface = make(map[string]interface{})
	} else {
		outputInterface = make([]interface{}, 0)
	}
	err := json.Unmarshal([]byte(inputJson), &outputInterface)
	require.NoErrorf(t, err, description)
	filteredOutput, err := util.SelectFilter(outputInterface, selectKeys)
	require.NoErrorf(t, err, description)
	filteredOutputBytes, err := json.Marshal(filteredOutput)
	require.NoErrorf(t, err, description)
	actualJson := string(filteredOutputBytes)
	require.JSONEqf(t, exepectedJson, actualJson, description)
}

func TestExampleSelectFilter(t *testing.T) {

	blocks := make([]models.Block, 2)
	for i := range blocks {
		block, err := generateBlock()
		require.NoError(t, err)
		blocks[i] = block
	}

	selectKeys := []string{
		"header.id",
		"payload.collection_guarantees.signature",
		"payload.block_seals.aggregated_approval_signatures.signer_ids",
		"payload.collection_guarantees.signer_indices",
		"execution_result.events.event_index",
		"something.nonexisting",
	}

	filteredBlock, err := util.SelectFilter(blocks, selectKeys)
	require.NoError(t, err)

	marshalled, err := json.MarshalIndent(filteredBlock, "", "\t")
	require.NoError(t, err)

	// enable to update test case if there is change in the models.Block struct
	// _ = ioutil.WriteFile("example_select_filter.json", marshalled, 0644)

	byteValue, err := ioutil.ReadFile("example_select_filter.json")
	require.NoError(t, err)

	require.Equal(t, string(byteValue), string(marshalled))
}

func generateBlock() (models.Block, error) {

	dummySignature := "abcdef0123456789"
	multipleDummySignatures := []string{dummySignature, dummySignature}
	dummyID := "abcd"
	dateString := "2021-11-20T11:45:26.371Z"
	t, err := time.Parse(time.RFC3339, dateString)
	if err != nil {
		return models.Block{}, err
	}

	return models.Block{
		Header: &models.BlockHeader{
			Id:                   dummyID,
			ParentId:             dummyID,
			Height:               "100",
			Timestamp:            t,
			ParentVoterSignature: dummySignature,
		},
		Payload: &models.BlockPayload{
			CollectionGuarantees: []models.CollectionGuarantee{
				{
					CollectionId:  "abcdef0123456789",
					SignerIndices: fmt.Sprintf("%x", []byte{1}),
					Signature:     dummySignature,
				},
			},
			BlockSeals: []models.BlockSeal{
				{
					BlockId:    dummyID,
					ResultId:   dummyID,
					FinalState: "final",
					AggregatedApprovalSignatures: []models.AggregatedSignature{
						{
							VerifierSignatures: multipleDummySignatures,
							SignerIds:          multipleDummySignatures,
						},
					},
				},
			},
		},
		ExecutionResult: &models.ExecutionResult{
			Id:      dummyID,
			BlockId: dummyID,
			Events: []models.Event{
				{
					Type_:            "type",
					TransactionId:    dummyID,
					TransactionIndex: "1",
					EventIndex:       "2",
					Payload:          "payload",
				},
				{
					Type_:            "type",
					TransactionId:    dummyID,
					TransactionIndex: "1",
					EventIndex:       "3",
					Payload:          "payload",
				},
			},
			Links: &models.Links{
				Self: "link",
			},
		},
	}, nil
}
