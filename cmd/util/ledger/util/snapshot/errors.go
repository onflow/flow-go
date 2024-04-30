package snapshot

import "fmt"

type MigrationSnapshotError struct {
	Err error
}

func NewMigrationSnapshotError(err error) *MigrationSnapshotError {
	return &MigrationSnapshotError{Err: err}
}

func (e *MigrationSnapshotError) Error() string {
	return fmt.Sprintf("failed to create snapshot for migration: %s", e.Err)
}

func (e *MigrationSnapshotError) Unwrap() error {
	return e.Err
}
