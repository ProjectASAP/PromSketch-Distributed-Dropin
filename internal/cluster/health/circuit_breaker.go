package health

import (
	"sync"
	"time"
)

// State represents the circuit breaker state
type State int

const (
	StateClosed   State = iota // Normal operation, requests flow through
	StateOpen                  // Circuit is open, requests are blocked
	StateHalfOpen              // Testing if the service has recovered
)

// CircuitBreaker implements the circuit breaker pattern for node health
type CircuitBreaker struct {
	state        State
	failures     int
	successes    int // consecutive successes in half-open state
	threshold    int
	timeout      time.Duration
	lastFailTime time.Time
	mu           sync.RWMutex
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(failureThreshold int, timeout time.Duration) *CircuitBreaker {
	return &CircuitBreaker{
		state:     StateClosed,
		threshold: failureThreshold,
		timeout:   timeout,
	}
}

// Allow returns true if the request should be allowed through.
// When the circuit is open and the timeout has elapsed, it atomically
// transitions to half-open and allows exactly one probe request.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return true
	case StateOpen:
		if time.Since(cb.lastFailTime) > cb.timeout {
			// Atomically transition to half-open; only one caller sees this
			cb.state = StateHalfOpen
			cb.successes = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	}
	return false
}

// RecordSuccess records a successful request
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.successes++

	if cb.state == StateHalfOpen {
		// After a few successes in half-open, close the circuit
		if cb.successes >= 2 {
			cb.state = StateClosed
			cb.successes = 0
		}
	}
}

// RecordFailure records a failed request
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.successes = 0
	cb.lastFailTime = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = StateOpen
	}
}

// State returns the current circuit breaker state
func (cb *CircuitBreaker) GetState() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failures = 0
	cb.successes = 0
}
