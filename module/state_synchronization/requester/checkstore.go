package requester

import (
	"errors"
	"fmt"

	"github.com/hashicorp/go-multierror"
	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-datastore"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	"github.com/onflow/flow-go/model/encoding/cbor"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/compressor"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/storage"
	"github.com/rs/zerolog"
)

type DatastoreChecker interface {
	Run(irrecoverable.SignalerContext, uint64) error
}

// DownloadExecutionDataFunc is a function that downloads the ExecutionData for a given blockID/height
type DownloadExecutionDataFunc func(irrecoverable.SignalerContext, flow.Identifier, uint64) error

type datastoreCheckerImpl struct {
	log     zerolog.Logger
	bs      network.BlobService
	eds     state_synchronization.ExecutionDataService
	headers storage.Headers
	results storage.ExecutionResults

	startBlockHeight      uint64
	downloadExecutionData DownloadExecutionDataFunc
}

// NewDatastoreChecker returns a DatastoreChecker that checks the datastore for missing/corrupt
// ExecutionData blobs
func NewDatastoreChecker(
	logger zerolog.Logger,
	blobservice network.BlobService,
	eds state_synchronization.ExecutionDataService,
	headers storage.Headers,
	results storage.ExecutionResults,
	startBlockHeight uint64,
	download DownloadExecutionDataFunc,
) DatastoreChecker {
	return &datastoreCheckerImpl{
		log:                   logger.With().Str("module", "check_datastore").Logger(),
		bs:                    blobservice,
		eds:                   eds,
		headers:               headers,
		results:               results,
		startBlockHeight:      startBlockHeight,
		downloadExecutionData: download,
	}
}

// Run checks the configured datastore for ExecutionData blobs associated with all heights between
// startBlockHeight and lastHeight. If any blobs are missing, they are redownloaded by calling then
// downloadExecutionData callback. Any blobs that are found to be corrupt are deleted and redownloaded.
// Any other error is unexpected and returned.
func (c *datastoreCheckerImpl) Run(ctx irrecoverable.SignalerContext, lastHeight uint64) error {
	if c.startBlockHeight >= lastHeight {
		c.log.Info().Msg("skipping datastore check. no blocks in datastore")
		return nil
	}

	c.log.Info().
		Uint64("start_height", c.startBlockHeight).
		Uint64("end_height", lastHeight).
		Msg("starting check")

	// Search from the start height to the lastHeight, and confirm data is still available for all
	// heights. All data should be present, otherwise data was deleted or corrupted on disk.
	for height := c.startBlockHeight; height <= lastHeight; height++ {
		if ctx.Err() != nil {
			return nil
		}

		header, err := c.headers.ByHeight(height)
		if err != nil {
			return fmt.Errorf("failed to get block for height %d: %w", height, err)
		}

		result, err := c.results.ByBlockID(header.ID())
		if err != nil {
			return fmt.Errorf("failed to lookup execution result for block %d: %w", header.ID(), err)
		}

		checkLogger := c.log.With().
			Uint64("height", height).
			Str("block_id", header.ID().String()).
			Str("execution_data_id", result.ExecutionDataID.String()).
			Logger()

		exists, err := c.checkExecutionData(ctx, result.ExecutionDataID, checkLogger)

		if err != nil {
			return err
		}

		if !exists {
			checkLogger.Debug().Msg("redownloading missing blob")
			// blocking download of any missing block that should exist in the datastore
			// TODO: ideally, this should push a job into the blockConsumer's queue for the worker
			// pool to consume. However, the jobqueue doesn't currently have a convenient way to
			// push work into the queue.
			err = c.downloadExecutionData(ctx, header.ID(), height)
			if err != nil && err != ctx.Err() {
				return fmt.Errorf("failed to redownload execution data for height %d: %w", height, err)
			}
		}
	}

	c.log.Info().
		Uint64("start_height", c.startBlockHeight).
		Uint64("end_height", lastHeight).
		Uint64("total_checked", lastHeight-c.startBlockHeight+1).
		Msg("datastore check successful")

	return nil
}

// checkExecutionData checks the datastore for a specific ExecutionData blob
func (c *datastoreCheckerImpl) checkExecutionData(ctx irrecoverable.SignalerContext, rootID flow.Identifier, logger zerolog.Logger) (bool, error) {
	invalidCIDs, ok := c.eds.Check(ctx, rootID)

	// Check returns a list of CIDs with the corresponding errors encountered while retrieving their
	// data from the local datastore.

	if ok {
		logger.Debug().Msg("check ok")
		return true, nil
	}

	// One or more of the blobs are invalid. Inspect the errors to determine if any are corrupt or
	// have unexpected errors. Delete any corrupt blobs, and record unexpected errors. When the
	// ExecutionData is requested again via the ExecutionDataService, any blobs that are missing
	// from the blobstore will be redownloaded, and any that exist are ignored.

	var errs *multierror.Error
	for _, invalidCID := range invalidCIDs {
		cid := invalidCID.Cid
		err := invalidCID.Err

		// Not Found. Missing blobs will be redownloaded
		if errors.Is(err, blockservice.ErrNotFound) {
			logger.Info().Str("cid", cid.String()).Msg("blob not found")
			continue
		}

		// The blob's hash didn't match. This is a special case where the data was corrupted on
		// disk. Delete and redownload.
		if errors.Is(err, blockstore.ErrHashMismatch) {
			logger.Info().Str("cid", cid.String()).Msg("blob corrupt. deleting")

			err := c.bs.DeleteBlob(ctx, cid)

			if err != nil {
				return false, fmt.Errorf("failed to delete corrupted CID %s from rootCID %s: %w", cid, rootID, err)
			}

			continue
		}

		// Record any other errors to return
		errs = multierror.Append(errs, err)
	}

	return false, errs.ErrorOrNil()
}

// LocalExecutionDataService returns an ExecutionDataService that's configured to use only the local
// datastore, and to rehash blobs on read. This is used to check the validity of blobs that exist in
// the local db.
func LocalExecutionDataService(ctx irrecoverable.SignalerContext, ds datastore.Batching, log zerolog.Logger) state_synchronization.ExecutionDataService {
	blobService := p2p.NewBlobService(ds, p2p.WithHashOnRead(true))
	blobService.Start(ctx)

	return state_synchronization.NewExecutionDataService(
		new(cbor.Codec),
		compressor.NewLz4Compressor(),
		blobService,
		metrics.NewNoopCollector(),
		log,
	)
}
