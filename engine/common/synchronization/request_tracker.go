package synchronization

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

type RequestSender interface {
	SendRequest(req *messages.RequestWrapper, target flow.Identifier) error
}

type RequestInfo struct {
	TargetID     flow.Identifier // the ID of the node that the request was sent to
	Request      interface{}     // the request payload
	ResponseChan chan interface{}
	Canceled     chan struct{}
}

type RequestTracker struct {
	mu sync.RWMutex
	// TODO: consider replacing this with sync.Map
	activeRequests map[uint64]*RequestInfo
	logger         zerolog.Logger
	requestSender  RequestSender
}

func NewRequestTracker(logger zerolog.Logger, requestSender RequestSender) *RequestTracker {
	return &RequestTracker{
		activeRequests: make(map[uint64]*RequestInfo),
		logger:         logger.With().Str("component", "request_tracker").Logger(),
		requestSender:  requestSender,
	}
}

func (r *RequestTracker) SendRequest(ctx context.Context, req interface{}, targetID flow.Identifier) (interface{}, error) {
	requestID := rand.Uint64()

	wrappedRequest := &messages.RequestWrapper{
		RequestID:      requestID,
		RequestPayload: req,
	}

	r.mu.Lock()

	respChan := make(chan interface{})
	canceled := make(chan struct{})
	r.activeRequests[requestID] = &RequestInfo{
		TargetID:     targetID,
		Request:      req,
		ResponseChan: respChan,
		Canceled:     canceled,
	}

	r.mu.Unlock()

	defer func() {
		close(canceled)
		r.mu.Lock()
		defer r.mu.Unlock()
		delete(r.activeRequests, requestID)
	}()

	err := r.requestSender.SendRequest(wrappedRequest, targetID)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	select {
	case resp := <-respChan:
		return resp, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (r *RequestTracker) ProcessResponse(resp *messages.ResponseWrapper, originID flow.Identifier) error {
	r.mu.RLock()

	requestInfo, ok := r.activeRequests[resp.RequestID]

	r.mu.RUnlock()

	if !ok {
		return ErrUnknownRequestID
	}

	if requestInfo.TargetID != originID {
		return &MismatchingResponseOriginError{
			RequestTarget:  requestInfo.TargetID,
			ResponseOrigin: originID,
		}
	}

	select {
	case requestInfo.ResponseChan <- resp.ResponsePayload:
		return nil
	case <-requestInfo.Canceled:
		return ErrRequestCanceled
	}
}

// ErrUnknownRequestID is returned when a response is received for an unknown request ID.
// This can happen if a matching request was never sent, or if the request has already timed out.
var ErrUnknownRequestID = errors.New("unknown request ID")

// ErrRequestCanceled is returned when a request is canceled during processing of the response.
// This can happen if the request times out after the RequestInfo has already been retrieved but
// before the response is sent, or if the response is a duplicate.
var ErrRequestCanceled = errors.New("request canceled")

// MismatchingResponseOriginError is returned when a response is received for a request, but
// the origin of the response does not match the target of the request.
type MismatchingResponseOriginError struct {
	RequestTarget  flow.Identifier
	ResponseOrigin flow.Identifier
}

func (e *MismatchingResponseOriginError) Error() string {
	return fmt.Sprintf("response origin %v does not match request target %v", e.ResponseOrigin, e.RequestTarget)
}
