// Package candidates derives learning candidates from attributed outcomes.
//
// Generation is deterministic and model-free: the same attributed results and
// policy always yield the same candidates. Every candidate carries provenance,
// a scope, and evidence, and is only a proposal; memory owns promotion.
package candidates

import (
	"fmt"
	"sort"
	"time"

	"github.com/greadee/aa/kernel/joblearn"
	"github.com/greadee/aa/kernel/joblearn/attribution"
)

// DefaultSource names the producer recorded in candidate provenance.
const DefaultSource = "aa-kernel/joblearn"

// Policy bounds candidate generation.
type Policy struct {
	// MinOutcomes is the minimum attributed outcomes for a scope to be learned.
	MinOutcomes int
	// MinWorkerOutcomes is the minimum outcomes for a worker routing hint.
	MinWorkerOutcomes int
	// SuccessThreshold is the success rate that yields a pattern.
	SuccessThreshold float64
	// FailureThreshold is the failure rate that yields a pitfall.
	FailureThreshold float64
	// StrategyThreshold is the success rate that yields a strategy.
	StrategyThreshold float64
	// CostThreshold is the mean normalized cost that yields a cost pitfall.
	CostThreshold float64
	// MinImprovement is the margin over the scope baseline for a routing hint.
	MinImprovement float64
}

// DefaultPolicy returns conservative generation thresholds.
func DefaultPolicy() Policy {
	return Policy{
		MinOutcomes:       5,
		MinWorkerOutcomes: 3,
		SuccessThreshold:  0.8,
		FailureThreshold:  0.5,
		StrategyThreshold: 1.0,
		CostThreshold:     0.75,
		MinImprovement:    0.1,
	}
}

// Validate checks the policy's bounds.
func (p Policy) Validate() error {
	if p.MinOutcomes < 0 || p.MinWorkerOutcomes < 0 {
		return invalid("outcome minimums must be >= 0")
	}
	for name, v := range map[string]float64{
		"successThreshold":  p.SuccessThreshold,
		"failureThreshold":  p.FailureThreshold,
		"strategyThreshold": p.StrategyThreshold,
		"costThreshold":     p.CostThreshold,
		"minImprovement":    p.MinImprovement,
	} {
		if v < 0 || v > 1 {
			return invalid("policy.%s = %v is outside [0,1]", name, v)
		}
	}
	return nil
}

// Generator derives candidates from attributed results.
type Generator struct {
	policy Policy
	source string
	now    func() time.Time
}

// New returns a generator with the given policy.
func New(policy Policy) *Generator {
	return &Generator{policy: policy, source: DefaultSource, now: time.Now}
}

// WithClock overrides the time source (useful for deterministic tests).
func (g *Generator) WithClock(now func() time.Time) *Generator {
	g.now = now
	return g
}

// WithSource overrides the provenance source.
func (g *Generator) WithSource(source string) *Generator {
	g.source = source
	return g
}

// Generate derives candidates from attributed results, sorted and deduplicated.
func (g *Generator) Generate(results []attribution.Result) ([]joblearn.Candidate, error) {
	if err := g.policy.Validate(); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, nil
	}
	var out []joblearn.Candidate

	roles, err := attribution.GroupBy(results, attribution.ByRole)
	if err != nil {
		return nil, err
	}
	for _, grp := range roles {
		if grp.Value == "" || grp.Summary.Outcomes < g.policy.MinOutcomes {
			continue
		}
		if successRate(grp.Summary) >= g.policy.SuccessThreshold {
			out = append(out, g.build(joblearn.CandidatePattern, joblearn.LevelRole,
				scopeRole+grp.Value, grp.Summary, grpResults(results, attribution.ByRole, grp.Value),
				&joblearn.Applicability{Roles: []string{grp.Value}},
				fmt.Sprintf("Role %s succeeds reliably", grp.Value),
				fmt.Sprintf("%d of %d attributed outcomes succeeded", grp.Summary.Successes, grp.Summary.Outcomes),
				successRate(grp.Summary)))
		}
		if failureRate(grp.Summary) >= g.policy.FailureThreshold {
			out = append(out, g.build(joblearn.CandidatePitfall, joblearn.LevelRole,
				scopeRole+grp.Value, grp.Summary, grpResults(results, attribution.ByRole, grp.Value),
				&joblearn.Applicability{Roles: []string{grp.Value}},
				fmt.Sprintf("Role %s fails often", grp.Value),
				fmt.Sprintf("%d of %d attributed outcomes failed", grp.Summary.Failures, grp.Summary.Outcomes),
				failureRate(grp.Summary)))
		}
	}

	packages, err := attribution.GroupBy(results, attribution.ByWorkPackage)
	if err != nil {
		return nil, err
	}
	for _, grp := range packages {
		if grp.Value == "" || grp.Summary.Outcomes < g.policy.MinOutcomes {
			continue
		}
		evidence := grpResults(results, attribution.ByWorkPackage, grp.Value)
		if successRate(grp.Summary) >= g.policy.StrategyThreshold {
			out = append(out, g.build(joblearn.CandidateStrategy, joblearn.LevelTask,
				scopeWorkPackage+grp.Value, grp.Summary, evidence, nil,
				fmt.Sprintf("Strategy from work package %s", grp.Value),
				fmt.Sprintf("%d of %d attributed outcomes succeeded", grp.Summary.Successes, grp.Summary.Outcomes),
				successRate(grp.Summary)))
		}
		if grp.Summary.MeanCost >= g.policy.CostThreshold {
			out = append(out, g.build(joblearn.CandidatePitfall, joblearn.LevelProject,
				scopeWorkPackage+grp.Value, grp.Summary, evidence, nil,
				fmt.Sprintf("Work package %s is high cost", grp.Value),
				fmt.Sprintf("mean normalized cost %.2f met the %.2f threshold", grp.Summary.MeanCost, g.policy.CostThreshold),
				clamp01(grp.Summary.MeanCost)))
		}
	}

	out = append(out, g.routing(results)...)

	return dedupSort(out), nil
}

// routing emits a per-trade hint for the worker that beats the trade baseline.
func (g *Generator) routing(results []attribution.Result) []joblearn.Candidate {
	trades, err := attribution.GroupBy(results, attribution.ByTrade)
	if err != nil {
		return nil
	}
	var out []joblearn.Candidate
	for _, trade := range trades {
		if trade.Value == "" || trade.Summary.Outcomes < g.policy.MinOutcomes {
			continue
		}
		cluster := grpResults(results, attribution.ByTrade, trade.Value)
		best, ok := bestWorker(cluster)
		if !ok || best.outcomes < g.policy.MinWorkerOutcomes {
			continue
		}
		margin := best.mean - trade.Summary.MeanOverall
		if margin < g.policy.MinImprovement {
			continue
		}
		out = append(out, g.build(joblearn.CandidateRouting, joblearn.LevelWorkforce,
			scopeTrade+trade.Value, trade.Summary, best.results,
			&joblearn.Applicability{Trades: []string{trade.Value}},
			fmt.Sprintf("Prefer worker %s for trade %s", best.worker, trade.Value),
			fmt.Sprintf("mean overall %.2f beat the trade baseline %.2f by %.2f", best.mean, trade.Summary.MeanOverall, margin),
			clamp01(margin)))
	}
	return out
}

type workerStat struct {
	worker   string
	mean     float64
	outcomes int
	results  []attribution.Result
}

// bestWorker returns the highest-mean worker, breaking ties by worker id.
func bestWorker(results []attribution.Result) (workerStat, bool) {
	byWorker := map[string][]attribution.Result{}
	for _, r := range results {
		byWorker[r.Attribution.Worker] = append(byWorker[r.Attribution.Worker], r)
	}
	var best workerStat
	found := false
	for worker, group := range byWorker {
		if worker == "" {
			continue
		}
		s := attribution.Summarize(group)
		stat := workerStat{worker: worker, mean: s.MeanOverall, outcomes: s.Outcomes, results: group}
		if !found || stat.mean > best.mean || (stat.mean == best.mean && stat.worker < best.worker) {
			best, found = stat, true
		}
	}
	return best, found
}

func (g *Generator) build(kind joblearn.CandidateKind, level joblearn.CandidateLevel, scope string,
	summary attribution.Summary, results []attribution.Result, applicability *joblearn.Applicability,
	title, content string, confidence float64) joblearn.Candidate {
	return joblearn.Candidate{
		Kind:          kind,
		Level:         level,
		Scope:         scope,
		Title:         title,
		Content:       content,
		Applicability: applicability,
		Evidence:      evidenceOf(results),
		Provenance: &joblearn.Provenance{
			Source:     g.source,
			ProducedAt: g.now().UTC().Format(time.RFC3339),
		},
		Confidence: clamp01(confidence),
	}
}

func grpResults(results []attribution.Result, dimension attribution.Dimension, value string) []attribution.Result {
	var out []attribution.Result
	for _, r := range results {
		if dimensionValue(r.Attribution, dimension) == value {
			out = append(out, r)
		}
	}
	return out
}

func dimensionValue(a joblearn.Attribution, dimension attribution.Dimension) string {
	switch dimension {
	case attribution.ByRole:
		return a.Role
	case attribution.ByTrade:
		return a.Trade
	case attribution.ByWorker:
		return a.Worker
	case attribution.ByWorkPackage:
		return a.WorkPackageID
	default:
		return ""
	}
}

// evidenceOf returns the unique references from results, sorted.
func evidenceOf(results []attribution.Result) []joblearn.Reference {
	seen := map[string]bool{}
	var out []joblearn.Reference
	for _, r := range results {
		for _, ref := range r.Attribution.Evidence {
			key := ref.Kind + "\x00" + ref.ID + "\x00" + ref.Version
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, ref)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// dedupSort merges duplicates by (kind, scope, title) and returns a stable order.
func dedupSort(candidates []joblearn.Candidate) []joblearn.Candidate {
	byKey := map[string]joblearn.Candidate{}
	for _, c := range candidates {
		key := string(c.Kind) + "\x00" + c.Scope + "\x00" + c.Title
		if existing, ok := byKey[key]; ok {
			if c.Confidence > existing.Confidence {
				existing.Confidence = c.Confidence
			}
			existing.Evidence = mergeRefs(existing.Evidence, c.Evidence)
			byKey[key] = existing
			continue
		}
		byKey[key] = c
	}
	out := make([]joblearn.Candidate, 0, len(byKey))
	for _, c := range byKey {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		if out[i].Scope != out[j].Scope {
			return out[i].Scope < out[j].Scope
		}
		return out[i].Title < out[j].Title
	})
	return out
}

func mergeRefs(a, b []joblearn.Reference) []joblearn.Reference {
	seen := map[string]bool{}
	var out []joblearn.Reference
	for _, ref := range append(append([]joblearn.Reference(nil), a...), b...) {
		key := ref.Kind + "\x00" + ref.ID + "\x00" + ref.Version
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, ref)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func successRate(s attribution.Summary) float64 {
	if s.Outcomes == 0 {
		return 0
	}
	return float64(s.Successes) / float64(s.Outcomes)
}

func failureRate(s attribution.Summary) float64 {
	if s.Outcomes == 0 {
		return 0
	}
	return float64(s.Failures) / float64(s.Outcomes)
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

// Scope prefixes.
const (
	scopeRole        = "role:"
	scopeTrade       = "trade:"
	scopeWorkPackage = "work_package:"
)

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", joblearn.ErrInvalid, fmt.Sprintf(format, args...))
}
