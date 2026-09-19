// Package gate governs learned capabilities with evidence gates.
//
// Every capability is disabled by default and enables only when its policy is
// satisfied: enough attributed outcomes, a deterministic baseline, a backtest
// improvement over that baseline, a guaranteed deterministic fallback, and no
// bypass of a human gate or execution contract. The registry records the
// decision and the reasons, so a disabled capability is never a silent one.
package gate

import (
	"fmt"
	"sort"

	"github.com/greadee/aa/kernel/joblearn"
)

// Evidence is what a capability's gate evaluates.
type Evidence struct {
	// Outcomes is the number of attributed outcomes available.
	Outcomes int `json:"outcomes"`
	// Baseline reports whether a deterministic baseline is available.
	Baseline bool `json:"baseline"`
	// Improvement is the measured improvement over the baseline.
	Improvement float64 `json:"improvement"`
	// Fallback reports whether a deterministic fallback exists.
	Fallback bool `json:"fallback"`
	// NoGateBypass reports that the capability would not bypass a gate or
	// execution contract.
	NoGateBypass bool `json:"noGateBypass"`
}

// Decision is the outcome of evaluating one capability's gate.
type Decision struct {
	Capability joblearn.Capability      `json:"capability"`
	State      joblearn.CapabilityState `json:"state"`
	Policy     joblearn.GatePolicy      `json:"policy"`
	Evidence   Evidence                 `json:"evidence"`
	Reasons    []string                 `json:"reasons,omitempty"`
}

// Registry holds per-capability policies and the latest decisions.
type Registry struct {
	policies  map[joblearn.Capability]joblearn.GatePolicy
	decisions map[joblearn.Capability]Decision
}

// KnownCapabilities returns the gate-governed capabilities, sorted.
func KnownCapabilities() []joblearn.Capability {
	return []joblearn.Capability{
		joblearn.CapabilityConflict,
		joblearn.CapabilityEstimator,
		joblearn.CapabilityLearnedRouting,
		joblearn.CapabilityRecommender,
		joblearn.CapabilityRetrospective,
		joblearn.CapabilitySimilarity,
	}
}

// NewRegistry returns a registry with every known capability registered and
// disabled under the given default policy.
func NewRegistry(policy joblearn.GatePolicy) (*Registry, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	r := &Registry{
		policies:  make(map[joblearn.Capability]joblearn.GatePolicy),
		decisions: make(map[joblearn.Capability]Decision),
	}
	for _, c := range KnownCapabilities() {
		r.policies[c] = policy
		r.decisions[c] = Decision{Capability: c, State: joblearn.CapabilityDisabled, Policy: policy}
	}
	return r, nil
}

// SetPolicy overrides a registered capability's policy and resets its decision
// to disabled.
func (r *Registry) SetPolicy(c joblearn.Capability, p joblearn.GatePolicy) error {
	if _, ok := r.policies[c]; !ok {
		return fmt.Errorf("%w: capability %q", joblearn.ErrNotFound, c)
	}
	if err := p.Validate(); err != nil {
		return err
	}
	r.policies[c] = p
	r.decisions[c] = Decision{Capability: c, State: joblearn.CapabilityDisabled, Policy: p}
	return nil
}

// Policy returns a registered capability's policy.
func (r *Registry) Policy(c joblearn.Capability) (joblearn.GatePolicy, bool) {
	p, ok := r.policies[c]
	return p, ok
}

// Evaluate applies a capability's policy to evidence and records the decision.
// A capability enables only when no requirement is unmet.
func (r *Registry) Evaluate(c joblearn.Capability, e Evidence) (Decision, error) {
	p, ok := r.policies[c]
	if !ok {
		return Decision{}, fmt.Errorf("%w: capability %q", joblearn.ErrNotFound, c)
	}
	d := Decision{Capability: c, State: joblearn.CapabilityEnabled, Policy: p, Evidence: e}
	if e.Outcomes < p.MinOutcomes {
		d.Reasons = append(d.Reasons, fmt.Sprintf("insufficient outcomes: %d < %d", e.Outcomes, p.MinOutcomes))
	}
	if p.RequireBaseline && !e.Baseline {
		d.Reasons = append(d.Reasons, "no deterministic baseline")
	}
	if p.MinImprovement > 0 && e.Improvement < p.MinImprovement {
		d.Reasons = append(d.Reasons, fmt.Sprintf("improvement %.3f < required %.3f", e.Improvement, p.MinImprovement))
	}
	if p.RequireFallback && !e.Fallback {
		d.Reasons = append(d.Reasons, "no deterministic fallback")
	}
	if p.RequireNoGateBypass && !e.NoGateBypass {
		d.Reasons = append(d.Reasons, "would bypass a gate or execution contract")
	}
	if len(d.Reasons) > 0 {
		d.State = joblearn.CapabilityDisabled
	}
	r.decisions[c] = d
	return d, nil
}

// State returns a capability's current state. Unregistered or unevaluated
// capabilities are disabled.
func (r *Registry) State(c joblearn.Capability) joblearn.CapabilityState {
	if d, ok := r.decisions[c]; ok {
		return d.State
	}
	return joblearn.CapabilityDisabled
}

// Enabled reports whether a capability is enabled.
func (r *Registry) Enabled(c joblearn.Capability) bool {
	return r.State(c) == joblearn.CapabilityEnabled
}

// Decision returns the recorded decision for a capability.
func (r *Registry) Decision(c joblearn.Capability) (Decision, bool) {
	d, ok := r.decisions[c]
	return d, ok
}

// RequireEnabled returns ErrDisabled unless the capability is enabled.
func (r *Registry) RequireEnabled(c joblearn.Capability) error {
	if !r.Enabled(c) {
		return fmt.Errorf("%w: %s", joblearn.ErrDisabled, c)
	}
	return nil
}

// Decisions returns every recorded decision sorted by capability.
func (r *Registry) Decisions() []Decision {
	out := make([]Decision, 0, len(r.decisions))
	for _, d := range r.decisions {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Capability < out[j].Capability })
	return out
}
