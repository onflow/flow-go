package io

import (
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
		return fmt.Errorf("could not create output dir: %v", err)
	}

	err = ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("could not write file: %v", err)
	}

	return nil
}

// WriteText writes a byte array to the file at the given path.
func WriteText(path string, data []byte) error {
	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("could not write file: %v", err)
	}

	return nil
}
