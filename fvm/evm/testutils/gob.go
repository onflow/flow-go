package testutils

import (
	"encoding/gob"
	"os"
)

// Serialize function: saves map data to a file
func SerializeState(filename string, data map[string][]byte) error {
	// Create a file to save data
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use gob to encode data
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

// Deserialize function: reads map data from a file
func DeserializeState(filename string) (map[string][]byte, error) {
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Prepare the map to store decoded data
	var data map[string][]byte

	// Use gob to decode data
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// Serialize function: saves map data to a file
func SerializeAllocator(filename string, data map[string]uint64) error {
	// Create a file to save data
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Use gob to encode data
	encoder := gob.NewEncoder(file)
	err = encoder.Encode(data)
	if err != nil {
		return err
	}

	return nil
}

// Deserialize function: reads map data from a file
func DeserializeAllocator(filename string) (map[string]uint64, error) {
	// Open the file for reading
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Prepare the map to store decoded data
	var data map[string]uint64

	// Use gob to decode data
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return data, nil
}
