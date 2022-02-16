package commands

import "encoding/json"

func ConvertToInterfaceList(list interface{}) ([]interface{}, error) {
	var resultList []interface{}
	bytes, err := json.Marshal(list)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &resultList)
	return resultList, err
}

func ConvertToMap(object interface{}) (map[string]interface{}, error) {
	var result map[string]interface{}
	bytes, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(bytes, &result)
	return result, err
}
