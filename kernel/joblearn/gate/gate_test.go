package gate

import (
	"errors"
	"strings"
	"testing"

	"github.com/greadee/aa/kernel/joblearn"
)

func strictPolicy() joblearn.GatePolicy {
	return joblearn.GatePolicy{
		MinOutcomes:         5,
		MinImprovement:      0.1,
		RequireBaseline:     true,
		RequireFallback:     true,
		RequireNoGateBypass: true,
	}
}

func fullEvidence() Evidence {
	return Evidence{Outcomes: 5, Baseline: true, Improvement: 0.2, Fallback: true, NoGateBypass: true}
}

func TestKnownCapabilitiesSorted(t *testing.T) {
	caps := KnownCapabilities()
	if len(caps) != 6 {
		t.Fatalf("capabilities = %d, want 6", len(caps))
	}
	for i := 1; i < len(caps); i++ {
		if caps[i-1] >= caps[i] {
			t.Fatalf("capabilities not sorted: %v", caps)
		}
	}
}

func TestDefaultDisabled(t *testing.T) {
	r, err := NewRegistry(joblearn.DefaultGatePolicy())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	for _, c := range KnownCapabilities() {
		if r.Enabled(c) {
			t.Fatalf("%s enabled by default", c)
		}
		if r.State(c) != joblearn.CapabilityDisabled {
			t.Fatalf("%s state = %q", c, r.State(c))
		}
		d, ok := r.Decision(c)
		if !ok || d.State != joblearn.CapabilityDisabled {
			t.Fatalf("%s decision = %+v, %v", c, d, ok)
		}
	}
	if err := r.RequireEnabled(joblearn.CapabilitySimilarity); !errors.Is(err, joblearn.ErrDisabled) {
		t.Fatalf("err = %v, want ErrDisabled", err)
	}
}

func TestEvaluateEnablesOnEvidence(t *testing.T) {
	r, err := NewRegistry(strictPolicy())
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	d, err := r.Evaluate(joblearn.CapabilitySimilarity, fullEvidence())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d.State != joblearn.CapabilityEnabled || len(d.Reasons) != 0 {
		t.Fatalf("decision = %+v", d)
	}
	if !r.Enabled(joblearn.CapabilitySimilarity) {
		t.Fatal("capability not enabled")
	}
	if err := r.RequireEnabled(joblearn.CapabilitySimilarity); err != nil {
		t.Fatalf("RequireEnabled: %v", err)
	}
}

func TestEvaluateDisabledReasons(t *testing.T) {
	r, _ := NewRegistry(strictPolicy())
	d, err := r.Evaluate(joblearn.CapabilityEstimator, Evidence{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d.State != joblearn.CapabilityDisabled {
		t.Fatalf("empty evidence enabled: %+v", d)
	}
	for _, want := range []string{"insufficient outcomes", "baseline", "improvement", "fallback", "bypass"} {
		if !hasReason(d.Reasons, want) {
			t.Fatalf("missing reason %q in %v", want, d.Reasons)
		}
	}

	single := fullEvidence()
	single.NoGateBypass = false
	d, _ = r.Evaluate(joblearn.CapabilityEstimator, single)
	if len(d.Reasons) != 1 || !strings.Contains(d.Reasons[0], "bypass") {
		t.Fatalf("reasons = %v", d.Reasons)
	}

	below := fullEvidence()
	below.Outcomes = 4
	d, _ = r.Evaluate(joblearn.CapabilityEstimator, below)
	if !hasReason(d.Reasons, "insufficient outcomes") {
		t.Fatalf("reasons = %v", d.Reasons)
	}

	weak := fullEvidence()
	weak.Improvement = 0.05
	d, _ = r.Evaluate(joblearn.CapabilityEstimator, weak)
	if !hasReason(d.Reasons, "improvement") {
		t.Fatalf("reasons = %v", d.Reasons)
	}
}

func TestSetPolicyOverrideResets(t *testing.T) {
	r, _ := NewRegistry(strictPolicy())
	if _, err := r.Evaluate(joblearn.CapabilityRecommender, fullEvidence()); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !r.Enabled(joblearn.CapabilityRecommender) {
		t.Fatal("capability should be enabled before reset")
	}
	permissive := joblearn.GatePolicy{}
	if err := r.SetPolicy(joblearn.CapabilityRecommender, permissive); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}
	if r.Enabled(joblearn.CapabilityRecommender) {
		t.Fatal("SetPolicy did not reset the decision")
	}
	d, err := r.Evaluate(joblearn.CapabilityRecommender, Evidence{})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if d.State != joblearn.CapabilityEnabled {
		t.Fatalf("permissive policy did not enable: %+v", d)
	}
}

func TestUnknownCapability(t *testing.T) {
	r, _ := NewRegistry(joblearn.DefaultGatePolicy())
	if _, err := r.Evaluate("oracle", fullEvidence()); !errors.Is(err, joblearn.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if err := r.SetPolicy("oracle", joblearn.DefaultGatePolicy()); !errors.Is(err, joblearn.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	if r.State("oracle") != joblearn.CapabilityDisabled {
		t.Fatal("unknown capability should be disabled")
	}
	if _, ok := r.Policy("oracle"); ok {
		t.Fatal("unknown capability has a policy")
	}
}

func TestInvalidPolicy(t *testing.T) {
	if _, err := NewRegistry(joblearn.GatePolicy{MinOutcomes: -1}); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	r, _ := NewRegistry(joblearn.DefaultGatePolicy())
	if err := r.SetPolicy(joblearn.CapabilitySimilarity, joblearn.GatePolicy{MinImprovement: 2}); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestDecisionsSorted(t *testing.T) {
	r, _ := NewRegistry(strictPolicy())
	if _, err := r.Evaluate(joblearn.CapabilitySimilarity, fullEvidence()); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if _, err := r.Evaluate(joblearn.CapabilityConflict, fullEvidence()); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	decisions := r.Decisions()
	if len(decisions) != len(KnownCapabilities()) {
		t.Fatalf("decisions = %d", len(decisions))
	}
	for i := 1; i < len(decisions); i++ {
		if decisions[i-1].Capability >= decisions[i].Capability {
			t.Fatalf("decisions not sorted: %+v", decisions)
		}
	}
}

func hasReason(reasons []string, substr string) bool {
	for _, r := range reasons {
		if strings.Contains(r, substr) {
			return true
		}
	}
	return false
}
