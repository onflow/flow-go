package operation

import (
	"fmt"
	"io"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/storage"
)

type batchWriter struct {
	batch storage.Transaction
}

var _ PebbleReaderWriter = (*batchWriter)(nil)

func NewBatchWriter(batch storage.Transaction) pebble.Writer {
	return batchWriter{batch: batch}
}

// pebble.Writer implementation
func (b batchWriter) Set(key, value []byte, o *pebble.WriteOptions) error {
	return b.batch.Set(key, value)
}

func (b batchWriter) Delete(key []byte, o *pebble.WriteOptions) error {
	return b.batch.Delete(key)
}

func (b batchWriter) Apply(batch *pebble.Batch, o *pebble.WriteOptions) error {
	return fmt.Errorf("Apply not implemented")
}

func (b batchWriter) DeleteSized(key []byte, valueSize uint32, _ *pebble.WriteOptions) error {
	return fmt.Errorf("DeleteSized not implemented")
}

func (b batchWriter) LogData(data []byte, _ *pebble.WriteOptions) error {
	return fmt.Errorf("LogData not implemented")
}

func (b batchWriter) SingleDelete(key []byte, o *pebble.WriteOptions) error {
	return fmt.Errorf("SingleDelete not implemented")
}

func (b batchWriter) DeleteRange(start, end []byte, o *pebble.WriteOptions) error {
	return fmt.Errorf("DeleteRange not implemented")
}

func (b batchWriter) Merge(key, value []byte, o *pebble.WriteOptions) error {
	return fmt.Errorf("Merge not implemented")
}

func (b batchWriter) RangeKeySet(start, end, suffix, value []byte, o *pebble.WriteOptions) error {
	return fmt.Errorf("RangeKeySet not implemented")
}

func (b batchWriter) RangeKeyUnset(start, end, suffix []byte, opts *pebble.WriteOptions) error {
	return fmt.Errorf("RangeKeyUnset not implemented")
}

func (b batchWriter) RangeKeyDelete(start, end []byte, opts *pebble.WriteOptions) error {
	return fmt.Errorf("RangeKeyDelete not implemented")
}

// pebble.Reader implementation
func (b batchWriter) Get(key []byte) ([]byte, io.Closer, error) {
	return nil, nil, fmt.Errorf("Get not implemented")
}

func (b batchWriter) NewIter(o *pebble.IterOptions) (*pebble.Iterator, error) {
	return nil, fmt.Errorf("NewIter not implemented")
}

func (b batchWriter) Close() error {
	return fmt.Errorf("Close not implemented")
}
