package io

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
)

// ReadFile reads the file from path, if not found, it will print the absolute path, instead of
// relative path.
func ReadFile(path string) ([]byte, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("could not get absolution path: %w", err)
	}

	data, err := ioutil.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}

	return data, nil
}
