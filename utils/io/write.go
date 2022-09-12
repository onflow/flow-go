package io

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// WriteFile writes a byte array to the file at the given path.
// This method will also create the directory and file as needed.
func WriteFile(path string, data []byte) error {
	err := os.MkdirAll(filepath.Dir(path), 0755)
	if err != nil {
		return fmt.Errorf("could not create output dir: %w", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return fmt.Errorf("could not write file: %w", err)
	}

	return nil
}

// WriteText writes a byte array to the file at the given path.
func WriteText(path string, data []byte) error {
	err := os.WriteFile(path, data, 0644)
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

// TerminateOnFullDisk panics if the input error is (or wraps) the system "out of disk
// space" error. It's a no-op for any other error, or nil.
func TerminateOnFullDisk(err error) error {
	if err != nil && errors.Is(err, syscall.ENOSPC) {
		panic(fmt.Sprintf("disk full, terminating node: %s", err.Error()))
	}
	return err
}
