package joblearn

import (
	"errors"
	"testing"
)

func TestOutcomeValid(t *testing.T) {
	for _, ok := range []Outcome{
		OutcomeSucceeded, OutcomeFailed, OutcomePartial,
		OutcomeBlocked, OutcomeCancelled, OutcomeUnknown,
	} {
		if !ok.Valid() {
			t.Fatalf("%q should be valid", ok)
		}
	}
	if Outcome("done").Valid() {
		t.Fatal("unknown outcome reported valid")
	}
}

func TestAttributionValidate(t *testing.T) {
	valid := Attribution{WorkPackageID: "wp1", AttemptID: "a1", Outcome: OutcomeSucceeded}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	cases := []Attribution{
		{AttemptID: "a1", Outcome: OutcomeSucceeded},
		{WorkPackageID: "wp1", Outcome: OutcomeSucceeded},
		{WorkPackageID: "wp1", AttemptID: "a1", Outcome: "done"},
	}
	for i, c := range cases {
		if err := c.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d: err = %v, want ErrInvalid", i, err)
		}
	}
}

func TestScoreValidate(t *testing.T) {
	valid := Score{Version: MetricVersion, Success: 1, Cost: 0.25, Duration: 0.5, Retries: 0, Quality: 0.8, Overall: 0.7}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := (Score{Success: 1}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing version: err = %v, want ErrInvalid", err)
	}
	if err := (Score{Version: MetricVersion, Overall: 1.5}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("out of range: err = %v, want ErrInvalid", err)
	}
}

func TestCandidateValidate(t *testing.T) {
	valid := Candidate{Kind: CandidateStrategy, Level: LevelProject, Title: "t", Content: "c", Confidence: 0.5}
	if err := valid.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	cases := []Candidate{
		{Kind: "hunch", Level: LevelProject, Title: "t", Content: "c"},
		{Kind: CandidateStrategy, Level: "galaxy", Title: "t", Content: "c"},
		{Kind: CandidateStrategy, Level: LevelProject, Content: "c"},
		{Kind: CandidateStrategy, Level: LevelProject, Title: "t"},
		{Kind: CandidateStrategy, Level: LevelProject, Title: "t", Content: "c", Confidence: 2},
	}
	for i, c := range cases {
		if err := c.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("case %d: err = %v, want ErrInvalid", i, err)
		}
	}
}

func TestCandidateKindAndLevelValid(t *testing.T) {
	for _, k := range []CandidateKind{CandidatePattern, CandidatePitfall, CandidateStrategy, CandidateRouting} {
		if !k.Valid() {
			t.Fatalf("%q should be valid", k)
		}
	}
	for _, l := range []CandidateLevel{LevelSession, LevelTask, LevelProject, LevelRole, LevelWorkforce} {
		if !l.Valid() {
			t.Fatalf("%q should be valid", l)
		}
	}
	if CandidateKind("x").Valid() || CandidateLevel("x").Valid() {
		t.Fatal("unknown kind or level reported valid")
	}
}

func TestCapabilityValid(t *testing.T) {
	for _, c := range []Capability{
		CapabilitySimilarity, CapabilityRetrospective, CapabilityEstimator,
		CapabilityRecommender, CapabilityConflict, CapabilityLearnedRouting,
	} {
		if !c.Valid() {
			t.Fatalf("%q should be valid", c)
		}
	}
	if Capability("oracle").Valid() {
		t.Fatal("unknown capability reported valid")
	}
}

func TestGatePolicy(t *testing.T) {
	p := DefaultGatePolicy()
	if err := p.Validate(); err != nil {
		t.Fatalf("default policy invalid: %v", err)
	}
	if p.MinOutcomes <= 0 || !p.RequireBaseline || !p.RequireFallback || !p.RequireNoGateBypass {
		t.Fatalf("default policy is not conservative: %+v", p)
	}
	if err := (GatePolicy{MinOutcomes: -1}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("negative outcomes: err = %v, want ErrInvalid", err)
	}
	if err := (GatePolicy{MinImprovement: 2}).Validate(); !errors.Is(err, ErrInvalid) {
		t.Fatalf("out-of-range improvement: err = %v, want ErrInvalid", err)
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	for _, pair := range [][2]error{
		{ErrInvalid, ErrDisabled},
		{ErrDisabled, ErrNotReady},
		{ErrNotReady, ErrNoBaseline},
		{ErrNoBaseline, ErrNoFallback},
		{ErrNoFallback, ErrConflict},
		{ErrConflict, ErrBudget},
		{ErrBudget, ErrNotFound},
	} {
		if errors.Is(pair[0], pair[1]) {
			t.Fatalf("%v should not match %v", pair[0], pair[1])
		}
	}
	if !errors.Is(invalid("x"), ErrInvalid) {
		t.Fatal("invalid() does not wrap ErrInvalid")
	}
}

func TestFixedClockAdvancesByStep(t *testing.T) {
	c := NewFixedClock()
	first := c.Now()
	second := c.Now()
	if !second.After(first) {
		t.Fatalf("clock did not advance: %v then %v", first, second)
	}
	if second.Sub(first).Seconds() != 1 {
		t.Fatalf("step = %v, want 1s", second.Sub(first))
	}
}
