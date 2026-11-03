package relay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/protocol"
)

var (
	ErrPendingLimit       = errors.New("pending request limit reached")
	ErrPendingNotFound    = errors.New("pending request not found")
	ErrTunnelDisconnected = errors.New("tunnel disconnected")
)

type Outcome struct {
	Response protocol.HTTPResponse
	Err      error
}

type pendingRequest struct {
	tunnelID string
	result   chan Outcome
	cancel   context.CancelFunc
}

type PendingRequests struct {
	mu             sync.Mutex
	requests       map[string]*pendingRequest
	maxPending     int
	requestTimeout time.Duration
	generateID     idGenerator
}

func NewPendingRequests(maxPending int, requestTimeout time.Duration) (*PendingRequests, error) {
	if maxPending <= 0 {
		return nil, errors.New("max pending requests must be positive")
	}
	if requestTimeout <= 0 {
		return nil, errors.New("request timeout must be positive")
	}
	return &PendingRequests{
		requests:       make(map[string]*pendingRequest),
		maxPending:     maxPending,
		requestTimeout: requestTimeout,
		generateID:     func() (string, error) { return secureID(16) },
	}, nil
}

func (pending *PendingRequests) Register(parent context.Context, tunnelID string) (string, <-chan Outcome, error) {
	if parent == nil {
		return "", nil, errors.New("context is required")
	}
	if tunnelID == "" {
		return "", nil, errors.New("tunnel ID is required")
	}

	pending.mu.Lock()
	defer pending.mu.Unlock()
	if len(pending.requests) >= pending.maxPending {
		return "", nil, ErrPendingLimit
	}

	var requestID string
	for range 10 {
		candidate, err := pending.generateID()
		if err != nil {
			return "", nil, err
		}
		if _, exists := pending.requests[candidate]; !exists {
			requestID = candidate
			break
		}
	}
	if requestID == "" {
		return "", nil, errors.New("could not generate a unique request ID")
	}

	requestContext, cancel := context.WithTimeout(parent, pending.requestTimeout)
	request := &pendingRequest{tunnelID: tunnelID, result: make(chan Outcome, 1), cancel: cancel}
	pending.requests[requestID] = request
	go pending.watch(requestContext, requestID)
	return requestID, request.result, nil
}

func (pending *PendingRequests) Resolve(requestID string, response protocol.HTTPResponse) error {
	return pending.complete(requestID, Outcome{Response: response})
}

// ResolveFor prevents one connection from completing another tunnel's request.
func (pending *PendingRequests) ResolveFor(tunnelID, requestID string, response protocol.HTTPResponse) error {
	pending.mu.Lock()
	defer pending.mu.Unlock()
	request, ok := pending.requests[requestID]
	if !ok || request.tunnelID != tunnelID {
		return ErrPendingNotFound
	}
	delete(pending.requests, requestID)
	request.result <- Outcome{Response: response}
	request.cancel()
	return nil
}

func (pending *PendingRequests) Cancel(requestID string, reason error) error {
	if reason == nil {
		reason = context.Canceled
	}
	return pending.complete(requestID, Outcome{Err: reason})
}

func (pending *PendingRequests) CancelTunnel(tunnelID string, reason error) int {
	if reason == nil {
		reason = ErrTunnelDisconnected
	}

	pending.mu.Lock()
	defer pending.mu.Unlock()
	cancelled := 0
	for requestID, request := range pending.requests {
		if request.tunnelID != tunnelID {
			continue
		}
		delete(pending.requests, requestID)
		request.result <- Outcome{Err: reason}
		request.cancel()
		cancelled++
	}
	return cancelled
}

func (pending *PendingRequests) Count() int {
	pending.mu.Lock()
	defer pending.mu.Unlock()
	return len(pending.requests)
}

func (pending *PendingRequests) watch(ctx context.Context, requestID string) {
	<-ctx.Done()
	_ = pending.complete(requestID, Outcome{Err: ctx.Err()})
}

func (pending *PendingRequests) complete(requestID string, outcome Outcome) error {
	pending.mu.Lock()
	defer pending.mu.Unlock()
	request, exists := pending.requests[requestID]
	if !exists {
		return fmt.Errorf("%w: %s", ErrPendingNotFound, requestID)
	}
	delete(pending.requests, requestID)
	request.result <- outcome
	request.cancel()
	return nil
}
