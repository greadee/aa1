package attribution

import (
	"errors"
	"math"
	"reflect"
	"testing"

	"github.com/greadee/aa/kernel/joblearn"
	"github.com/greadee/aa/kernel/registry"
	"github.com/greadee/aa/kernel/telemetry"
)

func record(outcome string) telemetry.Record {
	return telemetry.Record{
		AttemptID:     "a1",
		AssignmentID:  "as1",
		WorkPackageID: "wp1",
		Outcome:       outcome,
		Calls:         3,
		Tokens:        1200,
		CostUSD:       2.5,
		DurationMS:    1800000,
	}
}

func TestMetaForWorker(t *testing.T) {
	w := registry.Worker{ID: "w1", Trade: "backend", Roles: []registry.Role{"Builder", "Architect"}}
	if got := MetaForWorker(w, ""); got.Role != "Architect" || got.Trade != "backend" || got.Worker != "w1" {
		t.Fatalf("MetaForWorker = %+v", got)
	}
	if got := MetaForWorker(w, "Builder"); got.Role != "Builder" {
		t.Fatalf("explicit role not kept: %+v", got)
	}
	if got := MetaForWorker(registry.Worker{ID: "w2", Trade: "qa"}, ""); got.Role != "" {
		t.Fatalf("role should stay empty: %+v", got)
	}
}

func TestParseOutcome(t *testing.T) {
	if o, err := ParseOutcome("succeeded"); err != nil || o != joblearn.OutcomeSucceeded {
		t.Fatalf("ParseOutcome = %q, %v", o, err)
	}
	if _, err := ParseOutcome("done"); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestAttributeLinks(t *testing.T) {
	meta := Meta{
		ProjectID:  "p1",
		Role:       "Builder",
		Trade:      "backend",
		Worker:     "w1",
		Sequence:   7,
		References: []joblearn.Reference{{Kind: "attempt", ID: "a1"}},
	}
	attr, err := Attribute(record("succeeded"), meta)
	if err != nil {
		t.Fatalf("Attribute: %v", err)
	}
	if attr.ProjectID != "p1" || attr.WorkPackageID != "wp1" || attr.AttemptID != "a1" ||
		attr.AssignmentID != "as1" || attr.Role != "Builder" || attr.Trade != "backend" ||
		attr.Worker != "w1" || attr.Outcome != joblearn.OutcomeSucceeded || attr.Sequence != 7 {
		t.Fatalf("attribution = %+v", attr)
	}
	meta.References[0].ID = "changed"
	if attr.Evidence[0].ID != "a1" {
		t.Fatal("evidence was aliased, not copied")
	}
}

func TestAttributeValidates(t *testing.T) {
	if _, err := Attribute(telemetry.Record{AttemptID: "a1", Outcome: "succeeded"}, Meta{}); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if _, err := Attribute(telemetry.Record{WorkPackageID: "wp1", Outcome: "succeeded"}, Meta{}); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if _, err := Attribute(telemetry.Record{WorkPackageID: "wp1", AttemptID: "a1", Outcome: "done"}, Meta{}); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestScoreComponents(t *testing.T) {
	meta := Meta{Retries: 1, Gates: []GateOutcome{{Gate: "tests", Passed: true}}}
	s, err := Score(record("succeeded"), meta, DefaultLimits())
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if s.Version != joblearn.MetricVersion {
		t.Fatalf("version = %q", s.Version)
	}
	if s.Success != 1 || s.Quality != 1 {
		t.Fatalf("success/quality = %v/%v", s.Success, s.Quality)
	}
	if s.Cost != 0.5 || s.Duration != 0.5 || s.Retries != 0.2 {
		t.Fatalf("cost/duration/retries = %v/%v/%v", s.Cost, s.Duration, s.Retries)
	}
	if math.Abs(s.Overall-0.88) > 1e-9 {
		t.Fatalf("overall = %v, want 0.88", s.Overall)
	}
}

func TestScoreOutcomeMapping(t *testing.T) {
	for outcome, want := range map[string]float64{
		"succeeded": 1,
		"partial":   0.5,
		"failed":    0,
		"blocked":   0,
		"cancelled": 0,
		"unknown":   0.25,
	} {
		s, err := Score(telemetry.Record{WorkPackageID: "wp1", AttemptID: "a1", Outcome: outcome}, Meta{}, DefaultLimits())
		if err != nil {
			t.Fatalf("%s: %v", outcome, err)
		}
		if s.Success != want {
			t.Fatalf("%s: success = %v, want %v", outcome, s.Success, want)
		}
	}
}

func TestScoreZeroLimitsClamp(t *testing.T) {
	meta := Meta{Retries: 100}
	s, err := Score(record("failed"), meta, Limits{})
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if s.Cost != 0 || s.Duration != 0 || s.Retries != 0 {
		t.Fatalf("zero limits should normalize to 0: %+v", s)
	}
	if err := (Limits{MaxCostUSD: -1}).Validate(); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestScoreClampsOverLimit(t *testing.T) {
	rec := record("succeeded")
	rec.CostUSD = 50
	rec.DurationMS = 100000000
	s, err := Score(rec, Meta{Retries: 50}, DefaultLimits())
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	if s.Cost != 1 || s.Duration != 1 || s.Retries != 1 {
		t.Fatalf("over-limit components should clamp to 1: %+v", s)
	}
}

func TestScoreIsDeterministic(t *testing.T) {
	rec := record("succeeded")
	meta := Meta{Retries: 1, Gates: []GateOutcome{{Gate: "tests", Passed: true}, {Gate: "human", Pending: true}}}
	a, err := Score(rec, meta, DefaultLimits())
	if err != nil {
		t.Fatalf("Score: %v", err)
	}
	b, _ := Score(rec, meta, DefaultLimits())
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("scores differ:\n%+v\n%+v", a, b)
	}
}

func TestDerive(t *testing.T) {
	res, err := Derive(record("failed"), Meta{Role: "Builder"}, DefaultLimits())
	if err != nil {
		t.Fatalf("Derive: %v", err)
	}
	if res.Attribution.Outcome != joblearn.OutcomeFailed || res.Score.Success != 0 {
		t.Fatalf("result = %+v", res)
	}
	if err := res.Attribution.Validate(); err != nil {
		t.Fatalf("attribution invalid: %v", err)
	}
	if err := res.Score.Validate(); err != nil {
		t.Fatalf("score invalid: %v", err)
	}
}

func TestSummarizeAndGroupBy(t *testing.T) {
	results := []Result{
		{Attribution: joblearn.Attribution{WorkPackageID: "wp1", Role: "Builder", Outcome: joblearn.OutcomeSucceeded}, Score: joblearn.Score{Overall: 0.9, Cost: 0.1, Duration: 0.2}},
		{Attribution: joblearn.Attribution{WorkPackageID: "wp2", Role: "Builder", Outcome: joblearn.OutcomeFailed}, Score: joblearn.Score{Overall: 0.3, Cost: 0.4, Duration: 0.6}},
		{Attribution: joblearn.Attribution{WorkPackageID: "wp3", Role: "Architect", Outcome: joblearn.OutcomeSucceeded}, Score: joblearn.Score{Overall: 0.6, Cost: 0.2, Duration: 0.4}},
	}
	all := Summarize(results)
	if all.Outcomes != 3 || all.Successes != 2 || all.Failures != 1 {
		t.Fatalf("summary = %+v", all)
	}
	if math.Abs(all.MeanOverall-0.6) > 1e-9 {
		t.Fatalf("mean overall = %v, want 0.6", all.MeanOverall)
	}
	if Summarize(nil).Outcomes != 0 {
		t.Fatal("empty summary should be zero")
	}

	groups, err := GroupBy(results, ByRole)
	if err != nil {
		t.Fatalf("GroupBy: %v", err)
	}
	if len(groups) != 2 || groups[0].Value != "Architect" || groups[1].Value != "Builder" {
		t.Fatalf("groups = %+v", groups)
	}
	if groups[1].Summary.Outcomes != 2 || groups[1].Summary.Successes != 1 || groups[1].Summary.Failures != 1 {
		t.Fatalf("builder group = %+v", groups[1])
	}
	if _, err := GroupBy(results, "galaxy"); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}

func TestSummarizeOrderIndependent(t *testing.T) {
	results := []Result{
		{Attribution: joblearn.Attribution{WorkPackageID: "wp1", AttemptID: "a1", Outcome: joblearn.OutcomeSucceeded}, Score: joblearn.Score{Overall: 0.9, Cost: 0.1, Duration: 0.2}},
		{Attribution: joblearn.Attribution{WorkPackageID: "wp2", AttemptID: "a2", Outcome: joblearn.OutcomeSucceeded}, Score: joblearn.Score{Overall: 0.3, Cost: 0.4, Duration: 0.6}},
	}
	forward := Summarize(results)
	reversed := Summarize([]Result{results[1], results[0]})
	if !reflect.DeepEqual(forward, reversed) {
		t.Fatalf("summary depends on input order: %+v vs %+v", forward, reversed)
	}
}
