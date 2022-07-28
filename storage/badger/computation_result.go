package badger

import (
	"errors"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/badger/operation"
)

type ComputationResultUploadStatus struct {
	db *badger.DB
}

func NewComputationResultUploadStatus(db *badger.DB) *ComputationResultUploadStatus {
	return &ComputationResultUploadStatus{
		db: db,
	}
}

func (c *ComputationResultUploadStatus) Upsert(executionDataId flow.Identifier,
	wasUploadCompleted bool) error {
	_, err := c.ByID(executionDataId)
	if errors.Is(err, storage.ErrNotFound) {
		return operation.RetryOnConflict(c.db.Update,
			operation.InsertComputationResultUploadStatus(executionDataId, wasUploadCompleted))
	} else if err != nil {
		return err
	}

	return operation.RetryOnConflict(c.db.Update,
		operation.UpdateComputationResultUploadStatus(executionDataId, wasUploadCompleted))
}

func (c *ComputationResultUploadStatus) GetAllIDs() ([]flow.Identifier, error) {
	ids := make([]flow.Identifier, 0)
	err := c.db.View(operation.GetAllComputationResultIDs(&ids))
	return ids, err
}

func (c *ComputationResultUploadStatus) ByID(computationResultID flow.Identifier) (bool, error) {
	var ret bool
	err := c.db.View(func(btx *badger.Txn) error {
		return operation.GetComputationResultUploadStatus(computationResultID, &ret)(btx)
	})
	if err != nil {
		return false, err
	}

	return ret, nil
}

func (c *ComputationResultUploadStatus) Remove(computationResultID flow.Identifier) error {
	return operation.RetryOnConflict(c.db.Update, func(btx *badger.Txn) error {
		return operation.RemoveComputationResultUploadStatus(computationResultID)(btx)
	})
}
