package commands

import (
	"encoding/json"
)

func ConvertToInterfaceList(list any) ([]any, error) {
	var resultList []any
	bytes, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &resultList)
	return resultList, err
}

func ConvertToMap(object any) (map[string]any, error) {
	var result map[string]any
	bytes, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &result)
	return result, err
}
