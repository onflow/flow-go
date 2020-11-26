package ping

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/onflow/flow-go/model/flow"
)

func readJson(jsonFileName string) (map[flow.Identifier]string, error) {

	// read the file
	byteValue, err := openAndReadFile(jsonFileName)
	if err != nil {
		return nil, err
	}

	// unmarshal json data
	result, err := unmarshalJson(byteValue)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", jsonFileName, err)
	}
	return result, nil
}

func openAndReadFile(fileName string) ([]byte, error) {
	//open
	jsonFile, err := os.Open(fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", fileName, err)
	}
	defer jsonFile.Close()

	// read
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", fileName, err)
	}
	return byteValue, nil
}

func unmarshalJson(jsonData []byte) (map[flow.Identifier]string, error) {
	var result map[flow.Identifier]string
	err := json.Unmarshal(jsonData, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}
