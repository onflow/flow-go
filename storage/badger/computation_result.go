package badger

import (
	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/model/flow"
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

func (c *ComputationResultUploadStatus) Upsert(blockID flow.Identifier,
	wasUploadCompleted bool) error {
	return operation.RetryOnConflict(c.db.Update, func(btx *badger.Txn) error {
		return operation.UpsertComputationResultUploadStatus(blockID, wasUploadCompleted)(btx)
	})
}

func (c *ComputationResultUploadStatus) GetIDsByUploadStatus(targetUploadStatus bool) ([]flow.Identifier, error) {
	ids := make([]flow.Identifier, 0)
	err := c.db.View(operation.GetBlockIDsByStatus(&ids, targetUploadStatus))
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
