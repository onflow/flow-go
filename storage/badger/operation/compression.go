package operation

import (
	"bytes"
	"context"
	"fmt"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/snappy"
	"github.com/rs/zerolog"
)

type KeyValue struct {
	key []byte
	val []byte
}

// TraverseKeyValues takes a channel and a badger DB and sends all key-values to the channel
func TraverseKeyValues(allKeyVals chan<- *KeyValue, db *badger.DB) error {
	defer func() {
		close(allKeyVals)
	}()

	return db.View(func(tx *badger.Txn) error {
		options := badger.DefaultIteratorOptions
		it := tx.NewIterator(options)
		defer it.Close()
		maxKey := makePrefix(codeMax)
		for it.Seek(nil); it.Valid(); it.Next() {
			item := it.Item()
			if bytes.Equal(item.Key(), maxKey) {
				continue
			}

			err := item.Value(func(val []byte) error {
				allKeyVals <- &KeyValue{
					key: item.Key(),
					val: val,
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

// CompressAndStore takes a channel of key-values and a badger DB and compresses and stores the key-values in the DB
func CompressAndStore(logger zerolog.Logger, ctx context.Context, keyvals <-chan *KeyValue, db *badger.DB, batchMaxLen int, batchMaxByteSize int) error {
	err := db.Update(InitMax)
	if err != nil {
		return fmt.Errorf("could not initialize max tracker: %w", err)
	}

	batch := db.NewWriteBatch()
	batchLen := 0
	batchByteSize := 0
	total := 0
	for kv := range keyvals {
		select {
		case <-ctx.Done():
			return nil
		default:
			valSize, err := batchWriteCompressed(kv)(batch)
			if err != nil {
				return err
			}
			total++
			batchLen++
			batchByteSize += valSize
			if batchLen >= batchMaxLen || batchByteSize >= batchMaxByteSize {
				logger.Info().
					Int("batch_count", batchLen).
					Int("batch_size", batchByteSize).
					Float64("avg_size", float64(batchByteSize)/float64(batchLen)).
					Msgf("flushing batch")

				err := batch.Flush()
				if err != nil {
					return err
				}
				batchLen = 0
				batchByteSize = 0
				// reset the batch
				batch = db.NewWriteBatch()
			}
		}
	}
	logger.Info().Msgf("finished processing %d key-values", total)
	return nil
}

func batchWriteCompressed(kv *KeyValue) func(writeBatch *badger.WriteBatch) (int, error) {
	return func(writeBatch *badger.WriteBatch) (int, error) {
		// update the maximum key size if the inserted key is bigger
		if uint32(len(kv.key)) > max {
			max = uint32(len(kv.key))
			err := SetMax(writeBatch)
			if err != nil {
				return 0, fmt.Errorf("could not update max tracker: %w", err)
			}
		}

		compressed := snappy.Encode(nil, kv.val)

		// persist the entity data into the DB
		err := writeBatch.Set(kv.key, compressed)
		if err != nil {
			return 0, err
		}
		return len(compressed), nil
	}
}
