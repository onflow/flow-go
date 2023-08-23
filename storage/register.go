package storage

import (
	"github.com/onflow/flow-go/model/flow"
	"io"
)

type Register interface {
	RegisterReader
	RegisterWriter
	io.Closer
}

type RegisterReader interface {
	Get(ID flow.RegisterID, height uint64) flow.RegisterEntry
}

type RegisterWriter interface {
	Store(entry flow.RegisterEntry, height uint64) error
}
