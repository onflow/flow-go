package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/onflow/flow-go/engine/access/rest/generated"
)

type SearchResult struct {
	Date        string      `json:"date"`
	IdCompany   int         `json:"idCompany"`
	Company     string      `json:"company"`
	IdIndustry  interface{} `json:"idIndustry"`
	Industry    string      `json:"industry"`
	IdContinent interface{} `json:"idContinent"`
	Continent   string      `json:"continent"`
	IdCountry   interface{} `json:"idCountry"`
	Country     string      `json:"country"`
	IdState     interface{} `json:"idState"`
	State       string      `json:"state"`
	IdCity      interface{} `json:"idCity"`
	City        string      `json:"city"`
}

func fieldSet(fields ...string) map[string]bool {
	set := make(map[string]bool, len(fields))
	for _, s := range fields {
		set[s] = true
	}
	return set
}

func filterStructx(astruct interface{}, structType reflect.Type, fields ...string) interface{} {

	fs := fieldSet(fields...)
	if reflect.TypeOf(astruct).Kind() != reflect.Struct {
		return nil
	}

	rt, rv := reflect.TypeOf(astruct), reflect.ValueOf(astruct)
	fmt.Println(reflect.TypeOf(rt))
	fmt.Println(reflect.TypeOf(rv))

	out := make(map[string]interface{}, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := structType.Field(i)
		fmt.Println(field)
		jsonKey := field.Tag.Get("json")
		fmt.Println(jsonKey)
		if fs[jsonKey] {
			fmt.Println(rv.Field(i))
			out[jsonKey] = rv.Field(i).Interface()
		}
	}
	return out

}
func filterStruct(rv reflect.Value, fields ...string) interface{} {

	//fmt.Println(reflect.TypeOf(rv).String())
	//fmt.Println(reflect.TypeOf(rv).String()=="reflect.Value")
	fs := fieldSet(fields...)
	rt := rv.Type()

	out := make(map[string]interface{}, rt.NumField())
	for i := 0; i < rt.NumField(); i++ {
		field := rt.Field(i)
		fmt.Println(field.Type.Kind())
		switch field.Type.Kind() {
		case reflect.Slice: return SelectFields(field, fields...)
		case reflect.Struct: return SelectFields(field, fields...)
		}
		jsonKey := field.Tag.Get("json")
		//fmt.Println(jsonKey)
		if fs[jsonKey] {
			fmt.Println(rv.Field(i))
			out[jsonKey] = rv.Field(i).Interface()
		}
	}
	return out

}

func SelectFields(obj interface{}, fields ...string) interface{} {

	switch reflect.TypeOf(obj).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(obj)
		out := make([]interface{}, s.Len())
		for i := 0; i < s.Len(); i++ {
			element := s.Index(i)
			out[i] = filterStruct(element, fields...)
		}
		return out
	case reflect.Struct:
		v := reflect.ValueOf(obj)
		//t := reflect.TypeOf(obj)
		return filterStruct(v, fields...)

	case reflect.Ptr:
		v := reflect.ValueOf(obj)
		//t := reflect.TypeOf(obj)
		return filterStruct(v, fields...)

	default:
		panic("unexpected")
	}
}

func jsonPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return fmt.Sprintf("%s.%s", prefix, key)
}

func filter(json interface{}, prefix string, filterMap map[string]bool)  {
	switch t := json.(type) {
	case []interface{}:
		for _, item := range t {
			filter(item, prefix, filterMap)
		}
	case map[string]interface{}:
		for k,v := range t{
			key := jsonPath(prefix, k)
			switch value := v.(type) {
			case []interface{}:
				filter(value, key, filterMap)
			case map[string]interface{}:
				filter(value, key, filterMap)
			default:
				if filterMap[key] {
					delete(t, k)
					return
				}
			}
		}
	}
}

func main() {


	//result := SearchResult{
	//	Date:     "to be honest you should probably use a time.Time field here, just sayin",
	//	Industry: "rocketships",
	//	IdCity:   "interface{} is kinda inspecific, but this is the idcity field",
	//	City:     "New York Fuckin' City",
	//}

	//m, err := objx.FromJSON(json)
	//
	//
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
	filterMap["payload.collection_guarantees.signer_ids"] = true

	filter(*outputMap, "", filterMap)

	marshalled, err = json.MarshalIndent(outputMap, "", "\t")
	if err != nil {
		panic(err.Error())
	}
	fmt.Println(string(marshalled))
	//for k, v := range *outputMap {
	//	fmt.Println(k)
	//	fmt.Println(reflect.TypeOf(v))
	//}


	//
	//b1, err := json.MarshalIndent(SelectFields(generateBlock(), "date"), "", "  ")
	//if err != nil {
	//	panic(err.Error())
	//}
	//fmt.Println("---------")
	//fmt.Print(string(b1))
	//fmt.Println("---------")

	//blk := []generated.Block{generateBlock(), generateBlock()}
	//
	//b, err := json.MarshalIndent(SelectFields(blk, "date"), "", "  ")
	//if err != nil {
	//	panic(err.Error())
	//}
	//fmt.Print(string(b))
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
			},
			Links: &generated.Links{
				Self: "link",
			},
		},
	}
}
