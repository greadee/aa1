package joblearn

import "errors"

// Sentinel errors returned by joblearn components.
var (
	// ErrInvalid means the input did not satisfy the joblearn rules.
	ErrInvalid = errors.New("joblearn: invalid input")
	// ErrDisabled means a learned capability is disabled by its evidence gate.
	ErrDisabled = errors.New("joblearn: capability disabled")
	// ErrNotReady means there is not yet enough evidence to evaluate a capability.
	ErrNotReady = errors.New("joblearn: insufficient evidence")
	// ErrNoBaseline means a backtest was requested without a deterministic baseline.
	ErrNoBaseline = errors.New("joblearn: baseline required")
	// ErrNoFallback means a learned signal was requested without a fallback.
	ErrNoFallback = errors.New("joblearn: deterministic fallback required")
	// ErrConflict means two candidates conflict for the same scope.
	ErrConflict = errors.New("joblearn: conflicting candidates")
	// ErrBudget means an evaluation exceeded its performance budget.
	ErrBudget = errors.New("joblearn: performance budget exceeded")
	// ErrNotFound means the referenced candidate, capability, or object does not exist.
	ErrNotFound = errors.New("joblearn: not found")
)
