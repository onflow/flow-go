package uploader

import (
	"errors"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/storage"
)

// RetryableUploaderWrapper defines the interface for uploader that is retryable
type RetryableUploaderWrapper interface {
	Uploader
	RetryUpload() error
}

// BadgerRetryableUploaderWrapper is the BadgerDB based implementation to RetryableUploaderWrapper
type BadgerRetryableUploaderWrapper struct {
	uploader           *AsyncUploader
	execDataDownloader execution_data.Downloader
	unit               *engine.Unit
	metrics            module.ExecutionMetrics
	blocks             storage.Blocks
	commits            storage.Commits
	collections        storage.Collections
	events             storage.Events
	results            storage.ExecutionResults
	transactionResults storage.TransactionResults
	uploadStatusStore  storage.ComputationResultUploadStatus
}

func NewBadgerRetryableUploaderWrapper(
	uploader *AsyncUploader,
	blocks storage.Blocks,
	commits storage.Commits,
	collections storage.Collections,
	events storage.Events,
	results storage.ExecutionResults,
	transactionResults storage.TransactionResults,
	uploadStatusStore storage.ComputationResultUploadStatus,
	execDataDownloader execution_data.Downloader,
	metrics module.ExecutionMetrics) *BadgerRetryableUploaderWrapper {

	// check params
	if uploader == nil {
		log.Error().Msg("nil uploader passed in")
		return nil
	}
	if blocks == nil || commits == nil || collections == nil ||
		events == nil || results == nil || transactionResults == nil ||
		uploadStatusStore == nil || execDataDownloader == nil {
		log.Error().Msg("not all storage parameters are valid")
		return nil
	}

	// When Upload() is successful, the ComputationResult upload status in BadgerDB will be updated to true
	onCompleteCB := func(computationResult *execution.ComputationResult, err error) {
		if computationResult == nil || computationResult.ExecutableBlock == nil ||
			computationResult.ExecutableBlock.Block == nil {
			log.Warn().Msg("nil ComputationResult or nil ComputationResult.ExecutableBlock or " +
				"computationResult.ExecutableBlock.Block")
			return
		}

		blockID := computationResult.ExecutableBlock.Block.ID()
		if err != nil {
			log.Warn().Msgf("ComputationResults upload failed with BlockID %s", blockID.String())
			return
		}

		// Update upload status as Done(true)
		if err := uploadStatusStore.Upsert(blockID, true /*upload complete*/); err != nil {
			log.Warn().Msgf(
				"ComputationResults with BlockID %s failed to be updated on local disk. ERR: %s ",
				blockID.String(), err.Error())
		}

		metrics.ExecutionComputationResultUploaded()
	}

	uploader.SetOnCompleteCallback(onCompleteCB)

	return &BadgerRetryableUploaderWrapper{
		uploader:           uploader,
		execDataDownloader: execDataDownloader,
		unit:               engine.NewUnit(),
		metrics:            metrics,
		blocks:             blocks,
		commits:            commits,
		collections:        collections,
		events:             events,
		results:            results,
		transactionResults: transactionResults,
		uploadStatusStore:  uploadStatusStore,
	}
}

func (b *BadgerRetryableUploaderWrapper) Ready() <-chan struct{} {
	return b.uploader.Ready()
}

func (b *BadgerRetryableUploaderWrapper) Done() <-chan struct{} {
	return b.uploader.Done()
}

func (b *BadgerRetryableUploaderWrapper) Upload(computationResult *execution.ComputationResult) error {
	if computationResult == nil || computationResult.ExecutableBlock == nil ||
		computationResult.ExecutableBlock.Block == nil {
		return errors.New("ComputationResult or its ExecutableBlock(or its Block) is nil when Upload() is called")
	}

	// Before upload we store ComputationResult upload status to BadgerDB as false before upload is done.
	// It will be marked as true when upload completes.
	blockID := computationResult.ExecutableBlock.Block.ID()
	if err := b.uploadStatusStore.Upsert(blockID, false /*not completed*/); err != nil {
		log.Warn().Msgf("failed to store ComputationResult into local DB with BlockID %s", blockID)
	}

	return b.uploader.Upload(computationResult)
}

func (b *BadgerRetryableUploaderWrapper) RetryUpload() error {
	blockIDs, retErr := b.uploadStatusStore.GetIDsByUploadStatus(false /* not uploaded */)
	if retErr != nil {
		log.Error().Err(retErr).Msg("Failed to load the BlockID list of un-uploaded ComputationResult from local DB")
		return retErr
	}

	var wg sync.WaitGroup
	for _, blockID := range blockIDs {
		wg.Add(1)
		go func(blockID flow.Identifier) {
			defer wg.Done()

			log.Debug().Msgf("retrying upload for computation result of block %s", blockID.String())

			var cr_err error
			retComputationResult, err := b.reconstructComputationResult(blockID)
			if err != nil {
				err = cr_err
				log.Error().Err(err).Msgf(
					"failed to reconstruct ComputationResult with BlockID %s", blockID)
				return
			}

			// Do Upload
			if cr_err = b.uploader.Upload(retComputationResult); cr_err != nil {
				log.Error().Err(cr_err).Msgf(
					"Failed to re-upload ComputationResult with BlockID %s", blockID)
				retErr = cr_err
			} else {
				log.Debug().Msgf("computation result of block %s was successfully re-uploaded", blockID.String())
			}

			b.metrics.ExecutionComputationResultUploadRetried()
		}(blockID)
	}
	wg.Wait()

	// return latest occurred error
	return retErr
}

func (b *BadgerRetryableUploaderWrapper) reconstructComputationResult(
	blockID flow.Identifier) (*execution.ComputationResult, error) {

	// Get EDID from ExecutionResult in BadgerDB
	executionResult, err := b.results.ByBlockID(blockID)
	if err != nil {
		log.Error().Err(err).Msgf(
			"failed to retrieve ExecutionResult from Badger with BlockID %s", blockID.String())
		return nil, err
	}
	executionDataID := executionResult.ExecutionDataID

	// retrieving BlockExecutionData from EDS
	executionData, err := b.execDataDownloader.Get(b.unit.Ctx(), executionDataID)
	if executionData == nil || err != nil {
		log.Error().Err(err).Msgf(
			"failed to retrieve BlockExecutionData from EDS with ID %s", executionDataID.String())
		return nil, err
	}

	// retrieving events from local BadgerDB
	events, err := b.events.ByBlockID(blockID)
	if err != nil {
		log.Warn().Msgf(
			"failed to retrieve events for BlockID %s. Error: %s", blockID.String(), err.Error())
	}

	// retrieving Block from local BadgerDB
	block, err := b.blocks.ByID(blockID)
	if err != nil {
		log.Warn().Msgf(
			"failed to retrieve Block with BlockID %s. Error: %s", blockID.String(), err.Error())
	}

	// grabbing collections and guarantees from BadgerDB
	guarantees := make([]*flow.CollectionGuarantee, 0)
	if block != nil && block.Payload != nil {
		guarantees = block.Payload.Guarantees
	}

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection)
	for inx, guarantee := range guarantees {
		collectionID := guarantee.CollectionID
		collection, err := b.collections.ByID(collectionID)
		if err != nil {
			log.Warn().Msgf(
				"failed to retrieve collections with CollectionID %s. Error: %s", collectionID, err.Error())
			continue
		}

		completeCollections[collectionID] = &entity.CompleteCollection{
			Guarantee:    guarantees[inx],
			Transactions: collection.Transactions,
		}
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

	executableBlock := &entity.ExecutableBlock{
		Block:               block,
		CompleteCollections: completeCollections,
	}

	compRes := execution.NewEmptyComputationResult(executableBlock)

	eventsByTxIndex := make(map[int]flow.EventsList, 0)
	for _, event := range events {
		idx := int(event.TransactionIndex)
		eventsByTxIndex[idx] = append(eventsByTxIndex[idx], event)
	}

	lastChunk := len(completeCollections)
	lastCollection := compRes.CollectionExecutionResultAt(lastChunk)
	for i, txRes := range transactionResults {
		lastCollection.AppendTransactionResults(
			eventsByTxIndex[i],
			nil,
			nil,
			txRes,
		)
	}

	compRes.AppendCollectionAttestationResult(
		endState,
		endState,
		nil,
		flow.ZeroID,
		nil,
	)

	compRes.BlockExecutionData = executionData

	// for now we only care about fields in BlockData
	// Warning: this seems so broken just do the job, i only maintained previous behviour
	return compRes, nil
}
