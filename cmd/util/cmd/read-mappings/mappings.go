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

func WriteMegamappings(mappings map[string]delta.Mapping, filename string) error {
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
