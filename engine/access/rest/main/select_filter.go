package main

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/onflow/flow-go/engine/access/rest/generated"
)


func jsonPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return fmt.Sprintf("%s.%s", prefix, key)
}

func filterSlice(jsonSlice []interface{}, prefix string, filterMap map[string]bool) (bool, bool) {
	for _, item := range jsonSlice {
		switch itemAsType := item.(type) {
		case []interface{}:
			filterSlice(itemAsType, prefix, filterMap)
			if len(itemAsType) == 0{
				return false, true
			}
		case  map[string]interface{}:
			filterObject(itemAsType, prefix, filterMap)
			if len(itemAsType) == 0{
				jsonSlice = nil
				return false, true
			}
		default:
			return true, false
		}
	}
	return false, false
}

func filterObject(jsonStruct map[string]interface{}, prefix string, filterMap map[string]bool) {
	for key, item := range jsonStruct {
		newPrefix := jsonPath(prefix, key)
		switch itemAsType := item.(type) {
		case []interface{}:
			fmt.Println(key)
			fmt.Println(itemAsType)
			simpleSlice, sliceEmpty := filterSlice(itemAsType, newPrefix, filterMap)
			if simpleSlice {
				if !filterMap[newPrefix] {
					fmt.Printf("deleting %s\n", key)
					delete(jsonStruct, key)
					return
				}
			}
			fmt.Println(key)
			fmt.Println(itemAsType)
			if sliceEmpty {
				delete(jsonStruct, key)
			}
		case  map[string]interface{}:
			filterObject(itemAsType, newPrefix, filterMap)
			if len(itemAsType) == 0 {
				delete(jsonStruct, key)
			}
		default:
			if !filterMap[newPrefix] {
				fmt.Println(key)
				delete(jsonStruct, key)
			}
		}
	}
}


func main() {

	marshalled, err := json.MarshalIndent(generateBlock(), "", "\t")
	if err != nil {
		panic(err.Error())
	}

	//fmt.Println(string(marshalled))

	var outputMap = new(map[string]interface{})
	err = json.Unmarshal(marshalled, outputMap)
	if err != nil {
		panic(err.Error())
	}

	filterMap := map[string]bool{}
	filterMap["header.id"] = true
	filterMap["payload.collection_guarantees.signature"] = true
	filterMap["payload.block_seal.aggregated_approval_signatures.signer_ids"] = true
	//filterMap["payload.collection_guarantees.signer_ids"] = true
	filterMap["execution_result.events.event_index"] = true

	filterObject(*outputMap, "", filterMap)

	marshalled, err = json.MarshalIndent(outputMap, "", "\t")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(marshalled))
}

func generateBlock() generated.Block {

	dummySignature := "abcdef0123456789"
	multipleDummySignatures := []string{dummySignature, dummySignature}
	dummyID := "abcd"
	dateString := "2021-11-20T11:45:26.371Z"
	time, err := time.Parse(time.RFC3339, dateString)
	if err != nil {
		panic(fmt.Sprintf("error parsing date: %v", err))
	}

	return generated.Block{
		Header: &generated.BlockHeader{
			Id:                   dummyID,
			ParentId:             dummyID,
			Height:               100,
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
			Id: dummyID,
			BlockId: dummyID,
			Events: []generated.Event{
				{
					Type_: "type",
					TransactionId: dummyID,
					TransactionIndex: 1,
					EventIndex: 2,
					Payload: "payload",
				},
				{
					Type_: "type",
					TransactionId: dummyID,
					TransactionIndex: 1,
					EventIndex: 3,
					Payload: "payload",
				},
			},
			Links: &generated.Links{
				Self: "link",
			},
		},
	}
}
