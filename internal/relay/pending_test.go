package relay

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/SohamSDA/TunnelForge/internal/protocol"
)

func newTestPending(t *testing.T, maximum int, timeout time.Duration) *PendingRequests {
	t.Helper()
	pending, err := NewPendingRequests(maximum, timeout)
	if err != nil {
		t.Fatalf("NewPendingRequests() error: %v", err)
	}
	return pending
}

func TestPendingResolve(t *testing.T) {
	pending := newTestPending(t, 10, time.Second)
	requestID, result, err := pending.Register(context.Background(), "tunnel-1")
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	want := protocol.HTTPResponse{StatusCode: 201, Body: []byte("created")}
	if err := pending.Resolve(requestID, want); err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	outcome := <-result
	if outcome.Err != nil || outcome.Response.StatusCode != want.StatusCode || string(outcome.Response.Body) != string(want.Body) {
		t.Fatalf("outcome = %#v, want response %#v", outcome, want)
	}
	if pending.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", pending.Count())
	}
	if err := pending.Resolve(requestID, want); !errors.Is(err, ErrPendingNotFound) {
		t.Fatalf("second Resolve() error = %v, want ErrPendingNotFound", err)
	}
}

func TestPendingTimesOutAndCleansUp(t *testing.T) {
	pending := newTestPending(t, 10, 10*time.Millisecond)
	_, result, err := pending.Register(context.Background(), "tunnel-1")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case outcome := <-result:
		if !errors.Is(outcome.Err, context.DeadlineExceeded) {
			t.Fatalf("outcome error = %v, want DeadlineExceeded", outcome.Err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for pending request deadline")
	}
	if pending.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", pending.Count())
	}
}

func TestPendingParentCancellation(t *testing.T) {
	pending := newTestPending(t, 10, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	_, result, err := pending.Register(ctx, "tunnel-1")
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if outcome := <-result; !errors.Is(outcome.Err, context.Canceled) {
		t.Fatalf("outcome error = %v, want context.Canceled", outcome.Err)
	}
}

func TestPendingLimit(t *testing.T) {
	pending := newTestPending(t, 1, time.Second)
	if _, _, err := pending.Register(context.Background(), "tunnel-1"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pending.Register(context.Background(), "tunnel-1"); !errors.Is(err, ErrPendingLimit) {
		t.Fatalf("Register() error = %v, want ErrPendingLimit", err)
	}
}

func TestPendingCancelTunnel(t *testing.T) {
	pending := newTestPending(t, 10, time.Second)
	var tunnelOneResults []<-chan Outcome
	for range 3 {
		_, result, err := pending.Register(context.Background(), "tunnel-1")
		if err != nil {
			t.Fatal(err)
		}
		tunnelOneResults = append(tunnelOneResults, result)
	}
	_, otherResult, err := pending.Register(context.Background(), "tunnel-2")
	if err != nil {
		t.Fatal(err)
	}
	if got := pending.CancelTunnel("tunnel-1", nil); got != 3 {
		t.Fatalf("CancelTunnel() = %d, want 3", got)
	}
	for _, result := range tunnelOneResults {
		if outcome := <-result; !errors.Is(outcome.Err, ErrTunnelDisconnected) {
			t.Fatalf("outcome error = %v, want ErrTunnelDisconnected", outcome.Err)
		}
	}
	if pending.Count() != 1 {
		t.Fatalf("Count() = %d, want 1", pending.Count())
	}
	select {
	case outcome := <-otherResult:
		t.Fatalf("unrelated request completed: %#v", outcome)
	default:
	}
}

func TestPendingConcurrentResolve(t *testing.T) {
	const requests = 100
	pending := newTestPending(t, requests, time.Second)
	type registered struct {
		id     string
		result <-chan Outcome
	}
	registeredRequests := make([]registered, 0, requests)
	for index := range requests {
		id, result, err := pending.Register(context.Background(), fmt.Sprintf("tunnel-%d", index%5))
		if err != nil {
			t.Fatal(err)
		}
		registeredRequests = append(registeredRequests, registered{id: id, result: result})
	}

	var waitGroup sync.WaitGroup
	waitGroup.Add(requests)
	for index, request := range registeredRequests {
		go func() {
			defer waitGroup.Done()
			if err := pending.Resolve(request.id, protocol.HTTPResponse{StatusCode: 200, Body: []byte(fmt.Sprint(index))}); err != nil {
				t.Errorf("Resolve() error: %v", err)
			}
		}()
	}
	waitGroup.Wait()
	for _, request := range registeredRequests {
		if outcome := <-request.result; outcome.Err != nil {
			t.Errorf("outcome error: %v", outcome.Err)
		}
	}
	if pending.Count() != 0 {
		t.Fatalf("Count() = %d, want 0", pending.Count())
	}
}
