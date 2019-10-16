package main

import (
	"context"
	"errors"
	"log"
	"github.com/dapperlabs/flow-go/pkg/grpc/services/collect"
	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
)

var (
	errNotFound = errors.New("could not find transaction by key")
)

// Collector implements a mock version of Flow Collector Node that models a distributed set
type Collector struct {
	Data map[string]bool
}

var _ collect.CollectServiceServer = (*Collector)(nil)

// NewCollector is a constructor for collector
func NewCollector() *Collector {
	return &Collector{Data: make(map[string]bool, 0)}
}

// Ping to check the connection
func (w *Collector) Ping(ctx context.Context, pr *collect.PingRequest) (*collect.PingResponse, error) {
	return &collect.PingResponse{Address: []byte("Pong")}, nil
}

// SubmitTransaction registers a key into the data map of the collector node
func (w *Collector) SubmitTransaction(ctx context.Context, str *collect.SubmitTransactionRequest) (*collect.SubmitTransactionResponse, error) {
	log.Printf("Submitting %v", string(str.Transaction.GetScript()))
	w.Data[string(str.Transaction.GetScript())] = true
	return &collect.SubmitTransactionResponse{}, nil
}

// GetTransaction checks whether a key exists in the distributed storage
func (w *Collector) GetTransaction(ctx context.Context, gtr *collect.GetTransactionRequest) (*collect.GetTransactionResponse, error) {
	log.Printf("Getting %v", string(gtr.GetTransactionHash()))
	_, ok := w.Data[string(gtr.GetTransactionHash())]
	if ok != true {
		return &collect.GetTransactionResponse{}, errNotFound
	}

	return &collect.GetTransactionResponse{Transaction: &shared.Transaction{Script: gtr.GetTransactionHash()}}, nil
}

// SubmitCollection is remained unused but added to satisfy the collector interface
func (w *Collector) SubmitCollection(ctx context.Context, scr *collect.SubmitCollectionRequest) (*collect.SubmitCollectionResponse, error) {
	return &collect.SubmitCollectionResponse{}, nil
}

// GetCollection is remained unused but added to satisfy the collector interface
func (w *Collector) GetCollection(ctx context.Context, gcr *collect.GetCollectionRequest) (*collect.GetCollectionResponse, error) {
	return &collect.GetCollectionResponse{}, nil
}
