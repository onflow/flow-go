package storage

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	// Note: there is another not found error: badger.ErrKeyNotFound. The difference between
	// badger.ErrKeyNotFound and storage.ErrNotFound is that:
	// badger.ErrKeyNotFound is the error returned by the badger API.
	// Modules in storage/badger and storage/badger/operation package both
	// return storage.ErrNotFound for not found error
	ErrNotFound = errors.New("key not found")

	ErrAlreadyExists = errors.New("key already exists")
	ErrDataMismatch  = errors.New("data for key is different")
)

func ConvertStorageError(err error) error {
	if err == nil {
		return nil
	}

	if status.Code(err) == codes.NotFound {
		// Already converted
		return err
	}
	if errors.Is(err, ErrNotFound) {
		return status.Errorf(codes.NotFound, "not found: %v", err)
	}

	return status.Errorf(codes.Internal, "failed to find: %v", err)
}
