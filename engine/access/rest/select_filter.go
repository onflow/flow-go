package rest

import (
	"encoding/json"
	"fmt"
)

// SelectFilter selects the specified keys from the given object. The keys are in the json dot notation and must refer
// to leaf elements e.g. payload.collection_guarantees.signer_ids
func SelectFilter(object interface{}, selectKeys []string) (interface{}, error) {
	// avoid doing any work if no select keys provided
	if len(selectKeys) == 0 {
		return object, nil
	}

	marshalled, err := json.Marshal(object)
	if err != nil {
		return nil, err
	}

	var outputMap = new(interface{})
	err = json.Unmarshal(marshalled, outputMap)
	if err != nil {
		return nil, err
	}

	filter := sliceToMap(selectKeys)

	switch itemAsType := (*outputMap).(type) {
	case []interface{}:
		filterSlice(itemAsType, "", filter)
	case map[string]interface{}:
		filterObject(itemAsType, "", filter)
	}
	return *outputMap, nil
}

// filterObject filters a json struct. Prefix is the key prefix to use to find keys from the filterMap
// Leaf elements whose keys are not found in the filter map will be removed
func filterObject(jsonStruct map[string]interface{}, prefix string, filterMap map[string]bool) {
	for key, item := range jsonStruct {
		newPrefix := jsonPath(prefix, key)
		switch itemAsType := item.(type) {
		case []interface{}:
			// if the value of a key is a list, call filterSlice
			// e.g. { a : [ {b:1}, {b:2}...]
			itemAsType, simpleSlice := filterSlice(itemAsType, newPrefix, filterMap)
			// if the slice only had simple non-struct, non-list elements the filter it out if the key is not present
			// e.g. { a : [1,2,3]}
			if simpleSlice {
				if !filterMap[newPrefix] {
					delete(jsonStruct, key)
				}
			}
			// if after calling filterSlice the list is empty, then delete the key-value pair from the map
			if len(itemAsType) == 0 {
				delete(jsonStruct, key)
			}
		case map[string]interface{}:
			// if the value of a key is an object, then recurse
			// e.g.  { a :  { b: 1 } }
			filterObject(itemAsType, newPrefix, filterMap)
			if len(itemAsType) == 0 {
				delete(jsonStruct, key)
			}
		default:
			// if the value is a non-list,non-struct type then filter it out if the key is not present
			// e.g. { a : 1}
			if !filterMap[newPrefix] {
				delete(jsonStruct, key)
			}
		}
	}
}

// filterSlice filters a json slice. Prefix is the key prefix to use to find keys from the filterMap
// Leaf elements whose keys are not found in the filter map will be removed
// The function returns the modified slice and true if the slice only contains simple non-struct, non-list elements
func filterSlice(jsonSlice []interface{}, prefix string, filterMap map[string]bool) ([]interface{}, bool) {
	for _, item := range jsonSlice {
		switch itemAsType := item.(type) {
		case []interface{}:
			// if the slice has other slice as elements, recurse
			// e.g [[{b:1}, {b:2}...]]
			var sliceType bool
			itemAsType, sliceType = filterSlice(itemAsType, prefix, filterMap)
			if len(itemAsType) == 0 {
				// since all elements of the slice are the same, if one sub-slice has been filtered out, we can safely
				// remove all sub-slices and return (instead of iterating all slice elements)
				return nil, sliceType
			}
		case map[string]interface{}:
			// if the slice has structs as elements, call filterObject
			// e.g. [{a:1, b:2}, {a:3, b:4}]
			filterObject(itemAsType, prefix, filterMap)
			if len(itemAsType) == 0 {
				// since all elements of the slice are the same, if one struct element has been filtered out, we can safely
				// remove all struct elements and return (instead of iterating all slice elements)
				return nil, false
			}
		default:
			// if the elements are neither a slice nor a struct, then return the slice and true to indicate the slice has
			// only primitive elements
			// e.g. [1,2,3]
			return jsonSlice, true
		}
	}
	return jsonSlice, false
}

func jsonPath(prefix string, key string) string {
	if prefix == "" {
		return key
	}
	return fmt.Sprintf("%s.%s", prefix, key)
}

func sliceToMap(values []string) map[string]bool {
	valueMap := make(map[string]bool, len(values))
	for _, v := range values {
		valueMap[v] = true
	}
	return valueMap
}
