package candidates

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/greadee/aa/kernel/joblearn"
	"github.com/greadee/aa/kernel/joblearn/attribution"
)

func res(role, trade, worker, wp string, outcome joblearn.Outcome, overall, cost float64, n int) attribution.Result {
	return attribution.Result{
		Attribution: joblearn.Attribution{
			Role: role, Trade: trade, Worker: worker, WorkPackageID: wp, Outcome: outcome,
			Evidence: []joblearn.Reference{{Kind: "attempt", ID: fmt.Sprintf("%s-%d", wp, n)}},
		},
		Score: joblearn.Score{Version: joblearn.MetricVersion, Overall: overall, Cost: cost},
	}
}

func dataset() []attribution.Result {
	var out []attribution.Result
	for i := 0; i < 5; i++ {
		out = append(out, res("Builder", "backend", "w1", "wp1", joblearn.OutcomeSucceeded, 0.9, 0.1, i))
	}
	for i := 0; i < 3; i++ {
		out = append(out, res("Architect", "backend", "w2", "wp2", joblearn.OutcomeFailed, 0.3, 0.8, i))
	}
	for i := 3; i < 5; i++ {
		out = append(out, res("Architect", "backend", "w2", "wp2", joblearn.OutcomeSucceeded, 0.3, 0.8, i))
	}
	return out
}

func fixedClock() func() time.Time {
	return func() time.Time { return time.Unix(0, 0).UTC() }
}

func generate(t *testing.T, results []attribution.Result) []joblearn.Candidate {
	t.Helper()
	g := New(DefaultPolicy()).WithClock(fixedClock())
	out, err := g.Generate(results)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return out
}

func find(cs []joblearn.Candidate, kind joblearn.CandidateKind, scope string) (joblearn.Candidate, bool) {
	for _, c := range cs {
		if c.Kind == kind && c.Scope == scope {
			return c, true
		}
	}
	return joblearn.Candidate{}, false
}

func TestGenerateCandidates(t *testing.T) {
	cs := generate(t, dataset())
	if len(cs) != 5 {
		t.Fatalf("candidates = %d, want 5: %+v", len(cs), cs)
	}

	checks := []struct {
		kind       joblearn.CandidateKind
		scope      string
		level      joblearn.CandidateLevel
		confidence float64
	}{
		{joblearn.CandidatePattern, "role:Builder", joblearn.LevelRole, 1.0},
		{joblearn.CandidatePitfall, "role:Architect", joblearn.LevelRole, 0.6},
		{joblearn.CandidateStrategy, "work_package:wp1", joblearn.LevelTask, 1.0},
		{joblearn.CandidatePitfall, "work_package:wp2", joblearn.LevelProject, 0.8},
		{joblearn.CandidateRouting, "trade:backend", joblearn.LevelWorkforce, 0.3},
	}
	for _, want := range checks {
		c, ok := find(cs, want.kind, want.scope)
		if !ok {
			t.Fatalf("missing %s %s in %+v", want.kind, want.scope, cs)
		}
		if c.Level != want.level {
			t.Fatalf("%s level = %q, want %q", want.scope, c.Level, want.level)
		}
		if diff := c.Confidence - want.confidence; diff > 1e-9 || diff < -1e-9 {
			t.Fatalf("%s confidence = %v, want %v", want.scope, c.Confidence, want.confidence)
		}
		if err := c.Validate(); err != nil {
			t.Fatalf("%s invalid: %v", want.scope, err)
		}
		if len(c.Evidence) == 0 {
			t.Fatalf("%s has no evidence", want.scope)
		}
		if c.Provenance == nil || c.Provenance.Source != DefaultSource || c.Provenance.ProducedAt != "1970-01-01T00:00:00Z" {
			t.Fatalf("%s provenance = %+v", want.scope, c.Provenance)
		}
	}

	pattern, _ := find(cs, joblearn.CandidatePattern, "role:Builder")
	if pattern.Applicability == nil || !reflect.DeepEqual(pattern.Applicability.Roles, []string{"Builder"}) {
		t.Fatalf("pattern applicability = %+v", pattern.Applicability)
	}
	routing, _ := find(cs, joblearn.CandidateRouting, "trade:backend")
	if routing.Applicability == nil || !reflect.DeepEqual(routing.Applicability.Trades, []string{"backend"}) {
		t.Fatalf("routing applicability = %+v", routing.Applicability)
	}
}

func TestGenerateOrderIsStable(t *testing.T) {
	first := generate(t, dataset())
	shuffled := dataset()
	shuffled[0], shuffled[9] = shuffled[9], shuffled[0]
	second := generate(t, shuffled)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("generation depends on input order:\n%+v\n%+v", first, second)
	}
}

func TestGenerateDeterministicAcrossCalls(t *testing.T) {
	g := New(DefaultPolicy()).WithClock(fixedClock())
	a, err := g.Generate(dataset())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	b, err := g.Generate(dataset())
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatal("generation is not deterministic")
	}
}

func TestGenerateBelowThreshold(t *testing.T) {
	cs := generate(t, []attribution.Result{res("Builder", "backend", "w1", "wp1", joblearn.OutcomeSucceeded, 0.9, 0.1, 0)})
	if len(cs) != 0 {
		t.Fatalf("candidates = %+v, want none", cs)
	}
}

func TestGenerateNoRoutingWithoutMargin(t *testing.T) {
	var results []attribution.Result
	for i := 0; i < 5; i++ {
		results = append(results, res("Builder", "backend", "w1", "wp1", joblearn.OutcomeSucceeded, 0.5, 0.1, i))
		results = append(results, res("Builder", "backend", "w2", "wp2", joblearn.OutcomeSucceeded, 0.5, 0.1, i+5))
	}
	cs := generate(t, results)
	if _, ok := find(cs, joblearn.CandidateRouting, "trade:backend"); ok {
		t.Fatalf("routing emitted without a margin: %+v", cs)
	}
}

func TestPolicyValidate(t *testing.T) {
	if err := DefaultPolicy().Validate(); err != nil {
		t.Fatalf("default policy invalid: %v", err)
	}
	if err := (Policy{MinOutcomes: -1}).Validate(); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if err := (Policy{SuccessThreshold: 2}).Validate(); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	g := New(Policy{MinOutcomes: -1})
	if _, err := g.Generate(dataset()); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}
