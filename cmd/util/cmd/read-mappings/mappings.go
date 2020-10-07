package read_mappings

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/engine/execution/state/delta"
	"github.com/onflow/flow-go/storage/badger/operation"
)

func getMappingsFromDatabase(db *badger.DB) (map[string]delta.Mapping, error) {

	mappings := make(map[string]delta.Mapping)

	var found [][]*delta.LegacySnapshot
	err := db.View(operation.FindLegacyExecutionStateInteractions(func(interactions []*delta.LegacySnapshot) bool {

		for _, interaction := range interactions {
			for k, mapping := range interaction.Delta.ReadMappings {
				mappings[k] = mapping
			}
			for k, mapping := range interaction.Delta.WriteMappings {
				mappings[k] = mapping
			}
		}

		return false
	}, &found))

	return mappings, err
}

func writeMegamappings(mappings map[string]delta.Mapping, filename string) error {
	hexencodedMappings := make(map[string]delta.Mapping, len(mappings))
	for k, mapping := range mappings {
		hexencodedMappings[hex.EncodeToString([]byte(k))] = mapping
	}

	megaJson, _ := json.MarshalIndent(hexencodedMappings, "", "  ")
	err := ioutil.WriteFile(filename, megaJson, 0644)
	if err != nil {
		return fmt.Errorf("cannot write mappings file: %w", err)
	}

	return nil
}

func readMegamappings(filename string) (map[string]delta.Mapping, error) {
	var readMappings = map[string]delta.Mapping{}
	var hexencodedRead map[string]delta.Mapping

	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open mappings file: %w", err)
	}
	err = json.Unmarshal(bytes, &hexencodedRead)
	if err != nil {
		return nil, fmt.Errorf("cannot unmarshall mappings: %w", err)
	}

	for k, mapping := range hexencodedRead {
		decodeString, err := hex.DecodeString(k)
		if err != nil {
			return nil, fmt.Errorf("cannot decode key: %w", err)
		}
		readMappings[string(decodeString)] = mapping
	}

	return readMappings, nil
}
