package rest

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/access/rest/generated"
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
	filteredOutput, err := SelectFilter(outputInterface, selectKeys)
	require.NoErrorf(t, err, description)
	filteredOutputBytes, err := json.Marshal(filteredOutput)
	require.NoErrorf(t, err, description)
	actualJson := string(filteredOutputBytes)
	require.JSONEqf(t, exepectedJson, actualJson, description)
}

func ExampleSelectFilter() {

	blocks := make([]generated.Block, 2)
	for i := range blocks {
		block, err := generateBlock()
		if err != nil {
			fmt.Println(err)
			return
		}
		blocks[i] = block
	}

	selectKeys := []string{
		"header.id",
		"payload.collection_guarantees.signature",
		"payload.block_seals.aggregated_approval_signatures.signer_ids",
		"payload.collection_guarantees.signer_ids",
		"execution_result.events.event_index",
		"something.nonexisting",
	}

	filteredBlock, err := SelectFilter(blocks, selectKeys)
	if err != nil {
		fmt.Println(err)
		return
	}

	marshalled, err := json.MarshalIndent(filteredBlock, "", "\t")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(marshalled))
	// Output:
	//[
	//	{
	//		"execution_result": {
	//			"events": [
	//				{
	//					"event_index": "2"
	//				},
	//				{
	//					"event_index": "3"
	//				}
	//			]
	//		},
	//		"header": {
	//			"id": "abcd"
	//		},
	//		"payload": {
	//			"block_seals": [
	//				{
	//					"aggregated_approval_signatures": [
	//						{
	//							"signer_ids": [
	//								"abcdef0123456789",
	//								"abcdef0123456789"
	//							]
	//						}
	//					]
	//				}
	//			],
	//			"collection_guarantees": [
	//				{
	//					"signature": "abcdef0123456789",
	//					"signer_ids": [
	//						"abcdef0123456789",
	//						"abcdef0123456789"
	//					]
	//				}
	//			]
	//		}
	//	},
	//	{
	//		"execution_result": {
	//			"events": [
	//				{
	//					"event_index": "2"
	//				},
	//				{
	//					"event_index": "3"
	//				}
	//			]
	//		},
	//		"header": {
	//			"id": "abcd"
	//		},
	//		"payload": {
	//			"block_seals": [
	//				{
	//					"aggregated_approval_signatures": [
	//						{
	//							"signer_ids": [
	//								"abcdef0123456789",
	//								"abcdef0123456789"
	//							]
	//						}
	//					]
	//				}
	//			],
	//			"collection_guarantees": [
	//				{
	//					"signature": "abcdef0123456789",
	//					"signer_ids": [
	//						"abcdef0123456789",
	//						"abcdef0123456789"
	//					]
	//				}
	//			]
	//		}
	//	}
	//]
}

func generateBlock() (generated.Block, error) {

	dummySignature := "abcdef0123456789"
	multipleDummySignatures := []string{dummySignature, dummySignature}
	dummyID := "abcd"
	dateString := "2021-11-20T11:45:26.371Z"
	time, err := time.Parse(time.RFC3339, dateString)
	if err != nil {
		return generated.Block{}, err
	}

	return generated.Block{
		Header: &generated.BlockHeader{
			Id:                   dummyID,
			ParentId:             dummyID,
			Height:               "100",
			Timestamp:            time,
			ParentVoterSignature: dummySignature,
		},
		Payload: &generated.BlockPayload{
			CollectionGuarantees: []generated.CollectionGuarantee{
				{
					CollectionId: "abcdef0123456789",
					SignerIds:    multipleDummySignatures,
					Signature:    dummySignature,
				},
			},
			BlockSeals: []generated.BlockSeal{
				{
					BlockId:    dummyID,
					ResultId:   dummyID,
					FinalState: "final",
					AggregatedApprovalSignatures: []generated.AggregatedSignature{
						{
							VerifierSignatures: multipleDummySignatures,
							SignerIds:          multipleDummySignatures,
						},
					},
				},
			},
		},
		ExecutionResult: &generated.ExecutionResult{
			Id:      dummyID,
			BlockId: dummyID,
			Events: []generated.Event{
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
			Links: &generated.Links{
				Self: "link",
			},
		},
	}, nil
}
