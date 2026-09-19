// Package joblearn turns completed work into evidence and evidence into
// learning candidates.
//
// It attributes and scores outcomes deterministically, generates candidates
// with provenance, detects similar and conflicting candidates, gates every
// learned capability behind evidence, backtests against a deterministic
// baseline, and exposes learned routing as a non-authoritative hint with a
// fallback. It never promotes knowledge, never makes a learned signal
// authoritative, and never calls a model.
package joblearn

import (
	"fmt"

	v1 "github.com/greadee/aa/contracts/go/v1"
)

// Cross-module data structures live only in contracts; joblearn re-exports the
// ones it uses so callers can stay within one package boundary.
type (
	// Reference points at another contract object.
	Reference = v1.Reference
	// Provenance records where a record came from.
	Provenance = v1.Provenance
	// Applicability scopes a record to roles, trades, or languages.
	Applicability = v1.Applicability
)

// MetricVersion is the current scoring version. Scores are only comparable
// within one version.
const MetricVersion = "1.0"

// Outcome classifies how an attempt ended. It mirrors the telemetry contract.
type Outcome string

// Outcome values.
const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomePartial   Outcome = "partial"
	OutcomeBlocked   Outcome = "blocked"
	OutcomeCancelled Outcome = "cancelled"
	OutcomeUnknown   Outcome = "unknown"
)

// Valid reports whether the outcome is a known value.
func (o Outcome) Valid() bool {
	switch o {
	case OutcomeSucceeded, OutcomeFailed, OutcomePartial, OutcomeBlocked, OutcomeCancelled, OutcomeUnknown:
		return true
	default:
		return false
	}
}

// Attribution links one attempt's evidence to the work package, role, trade,
// and worker that produced it. Sequence orders attributions deterministically.
type Attribution struct {
	ProjectID     string      `json:"projectId,omitempty"`
	WorkPackageID string      `json:"workPackageId"`
	AttemptID     string      `json:"attemptId"`
	AssignmentID  string      `json:"assignmentId,omitempty"`
	Role          string      `json:"role,omitempty"`
	Trade         string      `json:"trade,omitempty"`
	Worker        string      `json:"worker,omitempty"`
	Outcome       Outcome     `json:"outcome"`
	Sequence      int         `json:"sequence"`
	Evidence      []Reference `json:"evidence,omitempty"`
}

// Validate checks the attribution's required fields.
func (a Attribution) Validate() error {
	if a.WorkPackageID == "" {
		return invalid("attribution.workPackageId is required")
	}
	if a.AttemptID == "" {
		return invalid("attribution.attemptId is required")
	}
	if !a.Outcome.Valid() {
		return invalid("attribution.outcome %q is unknown", a.Outcome)
	}
	return nil
}

// Score is a versioned, normalized evaluation of one attributed outcome. Every
// component is in [0, 1]; a lower cost, duration, or retry score means a better
// result.
type Score struct {
	Version  string  `json:"version"`
	Success  float64 `json:"success"`
	Cost     float64 `json:"cost"`
	Duration float64 `json:"duration"`
	Retries  float64 `json:"retries"`
	Quality  float64 `json:"quality"`
	Overall  float64 `json:"overall"`
}

// Validate checks that the score is versioned and every component is in range.
func (s Score) Validate() error {
	if s.Version == "" {
		return invalid("score.version is required")
	}
	for name, value := range map[string]float64{
		"success": s.Success, "cost": s.Cost, "duration": s.Duration,
		"retries": s.Retries, "quality": s.Quality, "overall": s.Overall,
	} {
		if value < 0 || value > 1 {
			return invalid("score.%s = %v is outside [0,1]", name, value)
		}
	}
	return nil
}

// CandidateKind names the kind of learning a candidate proposes.
type CandidateKind string

// Candidate kinds.
const (
	CandidatePattern  CandidateKind = "pattern"
	CandidatePitfall  CandidateKind = "pitfall"
	CandidateStrategy CandidateKind = "strategy"
	CandidateRouting  CandidateKind = "routing"
)

// Valid reports whether the candidate kind is known.
func (k CandidateKind) Valid() bool {
	switch k {
	case CandidatePattern, CandidatePitfall, CandidateStrategy, CandidateRouting:
		return true
	default:
		return false
	}
}

// CandidateLevel names the memory-hierarchy level a candidate applies to.
type CandidateLevel string

// Candidate levels.
const (
	LevelSession   CandidateLevel = "session"
	LevelTask      CandidateLevel = "task"
	LevelProject   CandidateLevel = "project"
	LevelRole      CandidateLevel = "role"
	LevelWorkforce CandidateLevel = "workforce"
)

// Valid reports whether the candidate level is known.
func (l CandidateLevel) Valid() bool {
	switch l {
	case LevelSession, LevelTask, LevelProject, LevelRole, LevelWorkforce:
		return true
	default:
		return false
	}
}

// Candidate is a proposed piece of knowledge awaiting validation. It is never
// authoritative; only memory promotes knowledge.
type Candidate struct {
	Kind          CandidateKind  `json:"kind"`
	Level         CandidateLevel `json:"level"`
	Scope         string         `json:"scope,omitempty"`
	Title         string         `json:"title"`
	Content       string         `json:"content"`
	Applicability *Applicability `json:"applicability,omitempty"`
	Evidence      []Reference    `json:"evidence,omitempty"`
	Provenance    *Provenance    `json:"provenance,omitempty"`
	Confidence    float64        `json:"confidence"`
}

// Validate checks the candidate's required fields and confidence range.
func (c Candidate) Validate() error {
	if !c.Kind.Valid() {
		return invalid("candidate.kind %q is unknown", c.Kind)
	}
	if !c.Level.Valid() {
		return invalid("candidate.level %q is unknown", c.Level)
	}
	if c.Title == "" {
		return invalid("candidate.title is required")
	}
	if c.Content == "" {
		return invalid("candidate.content is required")
	}
	if c.Confidence < 0 || c.Confidence > 1 {
		return invalid("candidate.confidence = %v is outside [0,1]", c.Confidence)
	}
	return nil
}

// Capability names a learned capability governed by an evidence gate.
type Capability string

// Learned capabilities.
const (
	CapabilitySimilarity     Capability = "similarity"
	CapabilityRetrospective  Capability = "retrospective"
	CapabilityEstimator      Capability = "estimator"
	CapabilityRecommender    Capability = "recommender"
	CapabilityConflict       Capability = "conflict"
	CapabilityLearnedRouting Capability = "learned_routing"
)

// Valid reports whether the capability is known.
func (c Capability) Valid() bool {
	switch c {
	case CapabilitySimilarity, CapabilityRetrospective, CapabilityEstimator,
		CapabilityRecommender, CapabilityConflict, CapabilityLearnedRouting:
		return true
	default:
		return false
	}
}

// CapabilityState is the evidence-gated enablement state of a capability.
// Capabilities are disabled until their gate is satisfied.
type CapabilityState string

// Capability states.
const (
	CapabilityDisabled CapabilityState = "disabled"
	CapabilityEnabled  CapabilityState = "enabled"
)

// GatePolicy is the evidence required before a capability may be enabled.
type GatePolicy struct {
	// MinOutcomes is the minimum number of attributed outcomes required.
	MinOutcomes int `json:"minOutcomes"`
	// MinImprovement is the minimum backtest improvement over the baseline.
	MinImprovement float64 `json:"minImprovement"`
	// RequireBaseline requires a backtest against a deterministic baseline.
	RequireBaseline bool `json:"requireBaseline"`
	// RequireFallback requires a deterministic fallback when the capability is
	// used.
	RequireFallback bool `json:"requireFallback"`
	// RequireNoGateBypass forbids bypassing a human gate or execution contract.
	RequireNoGateBypass bool `json:"requireNoGateBypass"`
}

// DefaultGatePolicy is the conservative policy: a real sample, a positive
// backtest, and a guaranteed fallback, with no gate bypass.
func DefaultGatePolicy() GatePolicy {
	return GatePolicy{
		MinOutcomes:         20,
		MinImprovement:      0.05,
		RequireBaseline:     true,
		RequireFallback:     true,
		RequireNoGateBypass: true,
	}
}

// Validate checks the policy's bounds.
func (p GatePolicy) Validate() error {
	if p.MinOutcomes < 0 {
		return invalid("gatePolicy.minOutcomes must be >= 0")
	}
	if p.MinImprovement < 0 || p.MinImprovement > 1 {
		return invalid("gatePolicy.minImprovement = %v is outside [0,1]", p.MinImprovement)
	}
	return nil
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, args...))
}
