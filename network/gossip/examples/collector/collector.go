package collector

import (
	"context"
	"errors"
	"log"

	"github.com/dapperlabs/flow-go/proto/sdk/entities"
	"github.com/dapperlabs/flow-go/proto/services/collection"
)

var (
	errNotFound = errors.New("could not find transaction by key")
)

// Collector implements a mock version of Flow Collector Node that models a distributed set
type Collector struct {
	Data map[string]bool
}

var _ collection.CollectServiceServer = (*Collector)(nil)

// NewCollector is a constructor for collection
func NewCollector() *Collector {
	return &Collector{Data: make(map[string]bool, 0)}
}

// Ping to check the connection
func (w *Collector) Ping(ctx context.Context, pr *collection.PingRequest) (*collection.PingResponse, error) {
	return &collection.PingResponse{Address: []byte("Pong")}, nil
}

// SubmitTransaction registers a key into the data map of the collection node
func (w *Collector) SubmitTransaction(ctx context.Context, str *collection.SubmitTransactionRequest) (*collection.SubmitTransactionResponse, error) {
	log.Printf("Submitting %v", string(str.Transaction.GetScript()))
	w.Data[string(str.Transaction.GetScript())] = true
	return &collection.SubmitTransactionResponse{}, nil
}

// GetTransaction checks whether a key exists in the distributed storage
func (w *Collector) GetTransaction(ctx context.Context, gtr *collection.GetTransactionRequest) (*collection.GetTransactionResponse, error) {
	log.Printf("Getting %v", string(gtr.Hash))
	_, ok := w.Data[string(gtr.Hash)]
	if ok != true {
		return &collection.GetTransactionResponse{}, errNotFound
	}

	return &collection.GetTransactionResponse{Transaction: &entities.Transaction{Script: gtr.Hash}}, nil
}

// SubmitCollection is remained unused but added to satisfy the collection interface
func (w *Collector) SubmitCollection(ctx context.Context, scr *collection.SubmitCollectionRequest) (*collection.SubmitCollectionResponse, error) {
	return &collection.SubmitCollectionResponse{}, nil
}

// GetCollection is remained unused but added to satisfy the collection interface
func (w *Collector) GetCollection(ctx context.Context, gcr *collection.GetCollectionRequest) (*collection.GetCollectionResponse, error) {
	return &collection.GetCollectionResponse{}, nil
}
