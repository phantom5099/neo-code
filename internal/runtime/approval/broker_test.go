package approval

import (
	"testing"
)

func TestBrokerNilReceiverBranches(t *testing.T) {
	t.Parallel()

	var broker *Broker
	if _, _, err := broker.Open(); err == nil {
		t.Fatalf("expected open on nil broker to fail")
	}
	if err := broker.Resolve("perm-1", DecisionAllowOnce); err == nil {
		t.Fatalf("expected resolve on nil broker to fail")
	}
	broker.Close("perm-1") // should be no-op
}

func TestBrokerOpenResolveCloseFlow(t *testing.T) {
	t.Parallel()

	broker := NewBroker()
	requestID, resultCh, err := broker.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if requestID == "" {
		t.Fatalf("expected non-empty request id")
	}

	if err := broker.Resolve(requestID, DecisionAllowSession); err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	select {
	case decision := <-resultCh:
		if decision != DecisionAllowSession {
			t.Fatalf("expected %q, got %q", DecisionAllowSession, decision)
		}
	default:
		t.Fatalf("expected decision to be delivered")
	}

	// duplicate submit should be idempotent and non-blocking
	if err := broker.Resolve(requestID, DecisionReject); err != nil {
		t.Fatalf("duplicate Resolve() error = %v", err)
	}

	broker.Close(requestID)
	if err := broker.Resolve(requestID, DecisionAllowOnce); err == nil {
		t.Fatalf("expected resolve after close to fail")
	}
}

func TestBrokerResolveUnknownRequest(t *testing.T) {
	t.Parallel()

	broker := NewBroker()
	if err := broker.Resolve("perm-missing", DecisionAllowOnce); err == nil {
		t.Fatalf("expected missing request error")
	}
}

func TestBrokerResolveChannelFullStillReturnsNil(t *testing.T) {
	t.Parallel()

	broker := NewBroker()
	requestID, resultCh, err := broker.Open()
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	// first resolve fills the buffered channel (capacity 1)
	if err := broker.Resolve(requestID, DecisionAllowOnce); err != nil {
		t.Fatalf("first Resolve() error = %v", err)
	}

	// simulate an already-submitted request whose channel is full,
	// to cover select default branch in Resolve.
	broker.mu.Lock()
	broker.pending[requestID] = &pendingRequest{resultCh: make(chan Decision, 1)}
	broker.pending[requestID].resultCh <- DecisionAllowOnce
	broker.mu.Unlock()

	if err := broker.Resolve(requestID, DecisionAllowSession); err != nil {
		t.Fatalf("Resolve() on full channel should still return nil, got %v", err)
	}

	// ensure original channel remains usable for read and not leaked behaviorally
	select {
	case <-resultCh:
	default:
	}
}
