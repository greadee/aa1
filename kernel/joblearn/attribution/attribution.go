// Package attribution links completed attempts to their work packages, roles,
// trades, and workers, and normalizes evidence into versioned scores.
//
// Attribution and scoring are pure functions of bounded evidence (telemetry
// and gate outcomes); the same input always yields the same result, and no
// model is called. Scores are candidate evidence only; they are never
// authoritative.
package attribution

import (
	"fmt"
	"sort"

	"github.com/greadee/aa/kernel/joblearn"
	"github.com/greadee/aa/kernel/registry"
	"github.com/greadee/aa/kernel/telemetry"
)

// GateOutcome is one gate's result for an attempt.
type GateOutcome struct {
	Gate    string `json:"gate"`
	Passed  bool   `json:"passed"`
	Pending bool   `json:"pending"`
	Reason  string `json:"reason,omitempty"`
}

// Meta is the attribution and evidence that telemetry does not carry.
type Meta struct {
	ProjectID  string
	Role       string
	Trade      string
	Worker     string
	Retries    int
	Sequence   int
	Gates      []GateOutcome
	References []joblearn.Reference
}

// MetaForWorker derives role, trade, and worker from a registered worker. When
// role is empty it falls back to the worker's lexicographically smallest role.
func MetaForWorker(w registry.Worker, role registry.Role) Meta {
	if role == "" {
		roles := append([]registry.Role(nil), w.Roles...)
		sort.Slice(roles, func(i, j int) bool { return roles[i] < roles[j] })
		if len(roles) > 0 {
			role = roles[0]
		}
	}
	return Meta{Role: string(role), Trade: string(w.Trade), Worker: string(w.ID)}
}

// Limits bound score normalization. A non-positive bound contributes zero.
type Limits struct {
	MaxCostUSD    float64
	MaxDurationMS int
	MaxRetries    int
}

// DefaultLimits returns conservative normalization bounds.
func DefaultLimits() Limits {
	return Limits{MaxCostUSD: 5.0, MaxDurationMS: 3600000, MaxRetries: 5}
}

// Validate checks that the bounds are non-negative.
func (l Limits) Validate() error {
	if l.MaxCostUSD < 0 || l.MaxDurationMS < 0 || l.MaxRetries < 0 {
		return invalid("limits must be >= 0")
	}
	return nil
}

// Result is one attributed outcome and its versioned score.
type Result struct {
	Attribution joblearn.Attribution `json:"attribution"`
	Score       joblearn.Score       `json:"score"`
}

// ParseOutcome maps a telemetry outcome string to a known outcome.
func ParseOutcome(s string) (joblearn.Outcome, error) {
	o := joblearn.Outcome(s)
	if !o.Valid() {
		return "", invalid("outcome %q is unknown", s)
	}
	return o, nil
}

// Attribute links a telemetry record to its work package, role, trade, and
// worker.
func Attribute(rec telemetry.Record, meta Meta) (joblearn.Attribution, error) {
	if rec.WorkPackageID == "" {
		return joblearn.Attribution{}, invalid("workPackageId is required")
	}
	if rec.AttemptID == "" {
		return joblearn.Attribution{}, invalid("attemptId is required")
	}
	outcome, err := ParseOutcome(rec.Outcome)
	if err != nil {
		return joblearn.Attribution{}, err
	}
	return joblearn.Attribution{
		ProjectID:     meta.ProjectID,
		WorkPackageID: rec.WorkPackageID,
		AttemptID:     rec.AttemptID,
		AssignmentID:  rec.AssignmentID,
		Role:          meta.Role,
		Trade:         meta.Trade,
		Worker:        meta.Worker,
		Outcome:       outcome,
		Sequence:      meta.Sequence,
		Evidence:      append([]joblearn.Reference(nil), meta.References...),
	}, nil
}

// Score normalizes a telemetry record and its gate outcomes into a versioned
// score. Cost, duration, and retry components are lower-is-better, so the
// overall score rewards the complement of each.
func Score(rec telemetry.Record, meta Meta, limits Limits) (joblearn.Score, error) {
	if err := limits.Validate(); err != nil {
		return joblearn.Score{}, err
	}
	outcome, err := ParseOutcome(rec.Outcome)
	if err != nil {
		return joblearn.Score{}, err
	}
	s := joblearn.Score{
		Version:  joblearn.MetricVersion,
		Success:  successScore(outcome),
		Cost:     normalize(rec.CostUSD, limits.MaxCostUSD),
		Duration: normalize(float64(rec.DurationMS), float64(limits.MaxDurationMS)),
		Retries:  normalize(float64(meta.Retries), float64(limits.MaxRetries)),
		Quality:  qualityScore(meta.Gates),
	}
	s.Overall = clamp01(0.5*s.Success +
		0.2*s.Quality +
		0.1*(1-s.Cost) +
		0.1*(1-s.Duration) +
		0.1*(1-s.Retries))
	if err := s.Validate(); err != nil {
		return joblearn.Score{}, err
	}
	return s, nil
}

// Derive attributes and scores one telemetry record.
func Derive(rec telemetry.Record, meta Meta, limits Limits) (Result, error) {
	attr, err := Attribute(rec, meta)
	if err != nil {
		return Result{}, err
	}
	score, err := Score(rec, meta, limits)
	if err != nil {
		return Result{}, err
	}
	return Result{Attribution: attr, Score: score}, nil
}

// Summary aggregates a set of attributed results.
type Summary struct {
	Outcomes     int
	Successes    int
	Failures     int
	MeanOverall  float64
	MeanCost     float64
	MeanDuration float64
}

// Summarize aggregates results deterministically. Results are accumulated in a
// canonical, content-derived order so floating-point sums do not depend on the
// order results were supplied in.
func Summarize(results []Result) Summary {
	var s Summary
	if len(results) == 0 {
		return s
	}
	for _, r := range canonical(results) {
		s.Outcomes++
		switch r.Attribution.Outcome {
		case joblearn.OutcomeSucceeded:
			s.Successes++
		case joblearn.OutcomeFailed:
			s.Failures++
		}
		s.MeanOverall += r.Score.Overall
		s.MeanCost += r.Score.Cost
		s.MeanDuration += r.Score.Duration
	}
	n := float64(len(results))
	s.MeanOverall /= n
	s.MeanCost /= n
	s.MeanDuration /= n
	return s
}

// canonical returns a copy of results sorted by a total, content-derived key.
// Two results with the same key are interchangeable, so the order is stable
// regardless of the input order.
func canonical(results []Result) []Result {
	type keyed struct {
		key string
		r   Result
	}
	ordered := make([]keyed, len(results))
	for i, r := range results {
		ordered[i] = keyed{key: sortKey(r), r: r}
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].key < ordered[j].key })
	out := make([]Result, len(results))
	for i, k := range ordered {
		out[i] = k.r
	}
	return out
}

func sortKey(r Result) string {
	a := r.Attribution
	s := r.Score
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s|%s|%d|%s|%s|%v|%v|%v|%v|%v|%v",
		a.ProjectID, a.WorkPackageID, a.AttemptID, a.AssignmentID, a.Role, a.Trade, a.Worker,
		a.Sequence, a.Outcome, s.Version, s.Success, s.Cost, s.Duration, s.Retries, s.Quality, s.Overall)
}

// Dimension is an attribution dimension to group by.
type Dimension string

// Grouping dimensions.
const (
	ByRole        Dimension = "role"
	ByTrade       Dimension = "trade"
	ByWorker      Dimension = "worker"
	ByWorkPackage Dimension = "work_package"
)

// Group is the aggregate of results sharing one dimension value.
type Group struct {
	Dimension string  `json:"dimension"`
	Value     string  `json:"value"`
	Summary   Summary `json:"summary"`
}

// GroupBy aggregates results along one dimension, sorted by value.
func GroupBy(results []Result, dimension Dimension) ([]Group, error) {
	key, err := keyFunc(dimension)
	if err != nil {
		return nil, err
	}
	grouped := map[string][]Result{}
	for _, r := range results {
		v := key(r.Attribution)
		grouped[v] = append(grouped[v], r)
	}
	values := make([]string, 0, len(grouped))
	for v := range grouped {
		values = append(values, v)
	}
	sort.Strings(values)
	out := make([]Group, 0, len(values))
	for _, v := range values {
		out = append(out, Group{Dimension: string(dimension), Value: v, Summary: Summarize(grouped[v])})
	}
	return out, nil
}

func keyFunc(dimension Dimension) (func(joblearn.Attribution) string, error) {
	switch dimension {
	case ByRole:
		return func(a joblearn.Attribution) string { return a.Role }, nil
	case ByTrade:
		return func(a joblearn.Attribution) string { return a.Trade }, nil
	case ByWorker:
		return func(a joblearn.Attribution) string { return a.Worker }, nil
	case ByWorkPackage:
		return func(a joblearn.Attribution) string { return a.WorkPackageID }, nil
	default:
		return nil, invalid("unknown dimension %q", dimension)
	}
}

func successScore(o joblearn.Outcome) float64 {
	switch o {
	case joblearn.OutcomeSucceeded:
		return 1
	case joblearn.OutcomePartial:
		return 0.5
	case joblearn.OutcomeUnknown:
		return 0.25
	default:
		return 0
	}
}

func qualityScore(gates []GateOutcome) float64 {
	if len(gates) == 0 {
		return 0.5
	}
	passed := 0
	for _, g := range gates {
		if g.Passed {
			passed++
		}
	}
	return float64(passed) / float64(len(gates))
}

func normalize(value, max float64) float64 {
	if max <= 0 {
		return 0
	}
	return clamp01(value / max)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", joblearn.ErrInvalid, fmt.Sprintf(format, args...))
}
