package noop

import "github.com/onflow/flow-go/storage"

type Iterator struct{}

var _ storage.Iterator = (*Iterator)(nil)

func (Iterator) First() bool {
	return false
}

func (Iterator) Valid() bool {
	return false
}

func (Iterator) Next() {}

func (Iterator) IterItem() storage.IterItem {
	return nil
}

func (Iterator) Close() error {
	return nil
}
