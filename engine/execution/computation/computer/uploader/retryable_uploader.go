package uploader

import (
	"errors"

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
	uploader           *AsyncUploader
	execDataService    state_synchronization.ExecutionDataService
	unit               *engine.Unit
	metrics            module.ExecutionMetrics
	blocks             storage.Blocks
	commits            storage.Commits
	transactionResults storage.TransactionResults
	uploadStatusStore  storage.ComputationResultUploadStatus
}

func NewBadgerRetryableUploader(
	uploader *AsyncUploader,
	blocks storage.Blocks,
	commits storage.Commits,
	transactionResults storage.TransactionResults,
	uploadStatusStore storage.ComputationResultUploadStatus,
	execDataService state_synchronization.ExecutionDataService,
	metrics module.ExecutionMetrics) *BadgerRetryableUploader {

	// check params
	if uploader == nil {
		log.Error().Msg("nil uploader passed in")
		return nil
	}
	if blocks == nil || commits == nil || transactionResults == nil ||
		uploadStatusStore == nil || execDataService == nil {
		log.Error().Msg("not all storage parameters are valid")
		return nil
	}

	// When Upload() is successful, the stored ComputationResult in BadgerDB will be removed.
	onCompleteCB := func(computationResult *execution.ComputationResult, err error) {
		if computationResult == nil {
			log.Warn().Msg("nil ComputationResult parameter")
			return
		}

		if err != nil {
			log.Warn().Msgf("ComputationResults upload failed with ID %s",
				computationResult.ExecutionDataID)
			return
		}

		// Update upload status as Done(true)
		if err := uploadStatusStore.Upsert(computationResult.ExecutionDataID,
			true /*upload complete*/); err != nil {
			log.Warn().Msgf(
				"ComputationResults with ID %s failed to be updated on local disk. ERR: %s ",
				computationResult.ExecutionDataID.String(), err.Error())
		}

		metrics.ExecutionComputationResultUploaded()
	}

	uploader.SetOnCompleteCallback(onCompleteCB)

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
	return b.uploader.Ready()
}

func (b *BadgerRetryableUploader) Done() <-chan struct{} {
	return b.uploader.Done()
}

func (b *BadgerRetryableUploader) Upload(computationResult *execution.ComputationResult) error {
	if computationResult == nil || computationResult.ExecutableBlock == nil {
		return errors.New("ComputationResult or its ExecutableBlock is nil when Upload() is called.")
	}

	// Before upload we store ComputationResult upload status to BadgerDB as false before upload is done.
	// It will be marked as true when upload completes.
	if err := b.uploadStatusStore.Upsert(computationResult.ExecutionDataID, false /*not completed*/); err != nil {
		log.Warn().Msgf("failed to store ComputationResult into local DB with ID %s",
			computationResult.ExecutionDataID)
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
			log.Error().Err(cr_err).Msgf(
				"Failed to load ComputationResult from local DB with ID %s", executionDataID)
			retErr = cr_err
			continue
		}

		if !wasUploadCompleted {
			retComputationResult, err := b.reconstructComputationResultFromEDS(executionDataID)
			if err != nil {
				err = cr_err
				log.Error().Err(err).Msgf(
					"failed to reconstruct ComputationResult with ID %s", executionDataID)
				continue
			}

			// Do Upload
			if cr_err = b.uploader.Upload(retComputationResult); cr_err != nil {
				log.Error().Err(cr_err).Msgf(
					"Failed to update ComputationResult with ID %s", executionDataID)
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
		log.Error().Err(err).Msgf(
			"failed to retrieve BlockData from EDS with ID %s", executionDataID.String())
		return nil, err
	}

	// grabbing TrieUpdates from ExecutionData
	trieUpdates := executionData.TrieUpdates
	if trieUpdates == nil {
		log.Warn().Msgf(
			"EDS returns nil trieUpdates for entry with ID %s", executionDataID.String())
	}

	// grabbing events from ExecutionData
	events := executionData.Events
	if events == nil {
		log.Warn().Msgf(
			"EDS returns nil events for entry with ID %s", executionDataID.String())
	}

	blockID := executionData.BlockID

	// retrieving Block from local BadgerDB
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		log.Warn().Msgf(
			"failed to retrieve Block with BlockID %s. Error: %s", blockID.String(), err.Error())
	}

	// grabbing collections from ExecutionData and collection guarantees from block
	// NOTE: we are assuming collection array stored on EDS has the same size\order
	//		 as the guarantees array stored in local storage.
	collections := executionData.Collections
	guarantees := make([]*flow.CollectionGuarantee, 0)
	if block != nil && block.Payload != nil {
		guarantees = block.Payload.Guarantees
	}

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection)
	if len(collections) == len(guarantees) {
		for inx, collection := range collections {
			completeCollections[collection.ID()] = &entity.CompleteCollection{
				Guarantee:    guarantees[inx],
				Transactions: collection.Transactions,
			}
		}
	} else {
		log.Warn().Msgf(
			"sizes of collection array and guarantee array don't match. BlockID %s", blockID.String())
	}

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeCollections,
	}

	// retrieving TransactionResults from BadgerDB
	transactionResults, err := b.transactionResults.ByBlockID(blockID)
	if err != nil {
		log.Warn().Msgf(
			"failed to retrieve TransactionResults with BlockID %s. Error: %s", blockID.String(), err.Error())
	}

	// retrieving CommitStatement from BadgerDB
	endState, err := b.commits.ByBlockID(blockID)
	if err != nil {
		log.Warn().Msgf("failed to retrieve StateCommitment with BlockID %s. Error: %s", blockID.String(), err.Error())
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
