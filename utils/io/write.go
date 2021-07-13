package io

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

// WriteFile writes a byte array to the file at the given path.
// This method will also create the directory and file as needed.
func WriteFile(path string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("could not create output dir: %w", err)
	}

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}

	return nil
}

// WriteText writes a byte array to the file at the given path.
func WriteText(path string, data []byte) error {
	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}

	return nil
}

// WriteJSON marshals the given interface into JSON and writes it to the given path
func WriteJSON(path string, data interface{}) error {
	bz, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("could not marshal json: %w", err)
	}

	return WriteFile(path, bz)
}
