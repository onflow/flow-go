package main

import (
	"github.com/onflow/flow-go/ledger/remote"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
)

// NewLedgerService creates a new ledger service (wrapper for remote.NewService)
func NewLedgerService(l ledger.Ledger, logger zerolog.Logger) *remote.Service {
	return remote.NewService(l, logger)
}
