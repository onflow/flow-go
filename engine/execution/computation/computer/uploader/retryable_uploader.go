package uploader

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/storage"
)

// RetryableUploader defines the interface for uploader that is retryable
type RetryableUploader interface {
	Uploader
	RetryUpload() error
}

// BadgerRetryableUploader is the BadgerDB based implementation to RetryableUploader
type BadgerRetryableUploader struct {
	uploader           Uploader
	execDataService    state_synchronization.ExecutionDataService
	unit               *engine.Unit
	metrics            module.ExecutionMetrics
	blocks             storage.Blocks
	commits            storage.Commits
	transactionResults storage.TransactionResults
	uploadStatusStore  storage.ComputationResultUploadStatus
}

func NewBadgerRetryableUploader(
	uploader Uploader,
	blocks storage.Blocks,
	commits storage.Commits,
	transactionResults storage.TransactionResults,
	uploadStatusStore storage.ComputationResultUploadStatus,
	execDataService state_synchronization.ExecutionDataService,
	metrics module.ExecutionMetrics) *BadgerRetryableUploader {

	// check storage\eds interface params
	if blocks == nil || commits == nil || transactionResults == nil ||
		uploadStatusStore == nil || execDataService == nil {
		panic("Not all storage parameters are valid")
	}

	// NOTE: only AsyncUploader is supported for now.
	switch uploader.(type) {
	case *AsyncUploader:
		// When Uploade() is successful, the stored ComputationResult in BadgerDB will be removed.
		onCompleteCB := func(computationResult *execution.ComputationResult, err error) {
			if err != nil {
				log.Warn().Msg(fmt.Sprintf("ComputationResults upload failed with ID %s",
					computationResult.ExecutionDataID))
				return
			}

			if computationResult == nil {
				log.Warn().Msg("nil ComputationResult parameter")
				return
			}

			// Removing existing upload status from BadgerDB
			if err = uploadStatusStore.Remove(computationResult.ExecutionDataID); err != nil {
				log.Warn().Msg(fmt.Sprintf(
					"ComputationResults with ID %s failed to be removed on local storage. ERR: %s ",
					computationResult.ExecutionDataID.String(), err.Error()))
			}
			// Store upload status as Done(true) anyway
			if err := uploadStatusStore.Store(computationResult.ExecutionDataID,
				true /*upload complete*/); err != nil {
				log.Warn().Msg(fmt.Sprintf(
					"ComputationResults with ID %s failed to be stored on local disk. ERR: %s ",
					computationResult.ExecutionDataID.String(), err.Error()))
			}

			metrics.ExecutionComputationResultUploaded()
		}
		uploader.(*AsyncUploader).SetOnCompleteCallback(onCompleteCB)
	}

	return &BadgerRetryableUploader{
		uploader:           uploader,
		execDataService:    execDataService,
		unit:               engine.NewUnit(),
		metrics:            metrics,
		blocks:             blocks,
		commits:            commits,
		transactionResults: transactionResults,
		uploadStatusStore:  uploadStatusStore,
	}
}

func (b *BadgerRetryableUploader) Ready() <-chan struct{} {
	switch b.uploader.(type) {
	case module.ReadyDoneAware:
		readyDoneAwareUploader := b.uploader.(module.ReadyDoneAware)
		return readyDoneAwareUploader.Ready()
	}
	return b.unit.Ready()
}

func (b *BadgerRetryableUploader) Done() <-chan struct{} {
	switch b.uploader.(type) {
	case module.ReadyDoneAware:
		readyDoneAwareUploader := b.uploader.(module.ReadyDoneAware)
		return readyDoneAwareUploader.Done()
	}
	return b.unit.Done()
}

func (b *BadgerRetryableUploader) Upload(computationResult *execution.ComputationResult) error {
	if computationResult == nil || computationResult.ExecutableBlock == nil {
		return errors.New("ComputationResult or its ExecutableBlock is nil when Upload() is called.")
	}

	// Before upload we store ComputationResult upload status to BadgerDB as false before upload is done.
	// It will be marked as true when upload completes.
	_, err := b.uploadStatusStore.ByID(computationResult.ExecutionDataID)
	if err == storage.ErrNotFound {
		if err := b.uploadStatusStore.Store(computationResult.ExecutionDataID, false /*not completed*/); err != nil {
			log.Warn().Msg(
				fmt.Sprintf("failed to store ComputationResult into local DB with ID %s",
					computationResult.ExecutionDataID))
		}
	}

	return b.uploader.Upload(computationResult)
}

func (b *BadgerRetryableUploader) RetryUpload() error {
	executionDataIDs, retErr := b.uploadStatusStore.GetAllIDs()
	if retErr != nil {
		log.Error().Err(retErr).Msg("Failed to load list of un-uploaded ComputationResult from local DB")
		return retErr
	}

	for _, executionDataID := range executionDataIDs {
		// Load stored ComputationResult upload status from BadgerDB
		wasUploadCompleted, cr_err := b.uploadStatusStore.ByID(executionDataID)
		if cr_err != nil {
			log.Error().Err(cr_err).Msg(
				fmt.Sprintf("Failed to load ComputationResult from local DB with ID %s", executionDataID))
			retErr = cr_err
			continue
		}

		if !wasUploadCompleted {
			retComputationResult, err := b.reconstructComputationResultFromEDS(executionDataID)
			if err != nil {
				err = cr_err
				log.Error().Err(err).Msg(
					fmt.Sprintf("failed to reconstruct ComputationResult with ID %s", executionDataID))
				continue
			}

			// Do Upload
			if cr_err = b.uploader.Upload(retComputationResult); cr_err != nil {
				log.Error().Err(cr_err).Msg(
					fmt.Sprintf("Failed to update ComputationResult with ID %s", executionDataID))
				retErr = cr_err
			}

			b.metrics.ExecutionComputationResultUploadRetried()
		}
	}

	// return latest occurred error
	return retErr
}

func (b *BadgerRetryableUploader) reconstructComputationResultFromEDS(
	executionDataID flow.Identifier) (*execution.ComputationResult, error) {

	// retrieving ExecutionData from EDS
	executionData, err := b.execDataService.Get(b.unit.Ctx(), executionDataID)
	if executionData == nil || err != nil {
		log.Error().Err(err).Msg(
			fmt.Sprintf("failed to retrieve BlockData from EDS with ID %s", executionDataID))
		return nil, err
	}

	// grabbing TrieUpdates from ExecutionData
	trieUpdates := executionData.TrieUpdates
	if trieUpdates == nil {
		log.Warn().Msg(
			fmt.Sprintf("EDS returns nil trieUpdates for entry with ID %s", executionDataID))
	}

	// grabbing events from ExecutionData
	events := executionData.Events
	if events == nil {
		log.Warn().Msg(
			fmt.Sprintf("EDS returns nil events for entry with ID %s", executionDataID))
	}

	blockID := executionData.BlockID

	// retrieving Block from local BadgerDB
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		log.Warn().Msg(
			fmt.Sprintf("failed to retrieve Block with BlockID %s. Error: %s", blockID, err.Error()))
	}

	// grabbing collections from ExecutionData and collection guarantees from block
	// NOTE: we are assuming collection array stored on EDS has the same size\order
	//		 as the guarantees array stored in local storage.
	collections := executionData.Collections
	guarantees := block.Payload.Guarantees

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection)
	if len(collections) == len(guarantees) {
		for inx, collection := range collections {
			completeCollections[collection.ID()] = &entity.CompleteCollection{
				Guarantee:    guarantees[inx],
				Transactions: collection.Transactions,
			}
		}
	} else {
		log.Warn().Msg(
			fmt.Sprintf("sizes of collection array and guarantee array don't match. BlockID %s", blockID))
	}

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeCollections,
	}

	// retrieving TransactionResults from BadgerDB
	transactionResults, err := b.transactionResults.ByBlockID(blockID)
	if err != nil {
		log.Warn().Msg(
			fmt.Sprintf("failed to retrieve TransactionResults with BlockID %s. Error: %s", blockID, err.Error()))
	}

	// retrieving CommitStatement from BadgerDB
	endState, err := b.commits.ByBlockID(blockID)
	if err != nil {
		log.Warn().Msg(
			fmt.Sprintf("failed to retrieve StateCommitment with BlockID %s. Error: %s", blockID, err.Error()))
	}
	stateCommitments := []flow.StateCommitment{
		endState,
	}

	// for now we only care about fields in BlockData on DPS
	return &execution.ComputationResult{
		ExecutableBlock:    executableBlock,
		Events:             events,
		ServiceEvents:      nil,
		TransactionResults: transactionResults,
		StateCommitments:   stateCommitments,
		Proofs:             nil,
		TrieUpdates:        trieUpdates,
		EventsHashes:       nil,
	}, nil
}
