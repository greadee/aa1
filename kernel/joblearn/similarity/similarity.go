// Package similarity clusters attributed outcomes deterministically and detects
// conflicting learning candidates.
//
// Similarity is feature-based and model-free: features are derived from role,
// trade, outcome, and normalized cost and duration, and compared with the
// Jaccard coefficient. Unique identifiers are excluded so outcomes can cluster
// across attempts, workers, and work packages. Clusters and conflicts are
// returned in a canonical order.
package similarity

import (
	"fmt"
	"sort"

	"github.com/greadee/aa/kernel/joblearn"
	"github.com/greadee/aa/kernel/joblearn/attribution"
)

// DefaultThreshold is the similarity at which attributed outcomes cluster.
const DefaultThreshold = 0.6

// Features returns the deterministic feature set of an attributed outcome.
// Attempt, assignment, work package, and worker identifiers are excluded so
// outcomes can cluster across them.
func Features(r attribution.Result) []string {
	a := r.Attribution
	var f []string
	add := func(prefix, value string) {
		if value != "" {
			f = append(f, prefix+":"+value)
		}
	}
	add("role", a.Role)
	add("trade", a.Trade)
	add("outcome", string(a.Outcome))
	add("cost", bucket(r.Score.Cost))
	add("duration", bucket(r.Score.Duration))
	sort.Strings(f)
	return dedup(f)
}

// Similarity returns the Jaccard similarity of two attributed outcomes in
// [0,1]. Two empty feature sets are treated as unrelated.
func Similarity(a, b attribution.Result) float64 {
	return jaccard(Features(a), Features(b))
}

// OutcomeCluster is a set of similar attributed outcomes.
type OutcomeCluster struct {
	Results []attribution.Result `json:"results"`
}

// Cluster groups outcomes connected by similarity at or above threshold. The
// result is deterministic and independent of input order.
func Cluster(results []attribution.Result, threshold float64) ([]OutcomeCluster, error) {
	if threshold < 0 || threshold > 1 {
		return nil, invalid("threshold %v is outside [0,1]", threshold)
	}
	if len(results) == 0 {
		return nil, nil
	}
	ordered := append([]attribution.Result(nil), results...)
	sort.Slice(ordered, func(i, j int) bool { return resultKey(ordered[i]) < resultKey(ordered[j]) })

	features := make([][]string, len(ordered))
	for i, r := range ordered {
		features[i] = Features(r)
	}

	visited := make([]bool, len(ordered))
	var clusters []OutcomeCluster
	for i := range ordered {
		if visited[i] {
			continue
		}
		visited[i] = true
		queue := []int{i}
		var members []int
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			members = append(members, cur)
			for j := range ordered {
				if visited[j] {
					continue
				}
				if jaccard(features[cur], features[j]) >= threshold {
					visited[j] = true
					queue = append(queue, j)
				}
			}
		}
		sort.Ints(members)
		cluster := OutcomeCluster{Results: make([]attribution.Result, 0, len(members))}
		for _, m := range members {
			cluster.Results = append(cluster.Results, ordered[m])
		}
		clusters = append(clusters, cluster)
	}
	return clusters, nil
}

func bucket(v float64) string {
	switch {
	case v < 0.34:
		return "low"
	case v < 0.67:
		return "mid"
	default:
		return "high"
	}
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	set := make(map[string]bool, len(a))
	for _, v := range a {
		set[v] = true
	}
	intersection := 0
	for _, v := range b {
		if set[v] {
			intersection++
		}
	}
	union := len(a) + len(b) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func dedup(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, v := range values {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func resultKey(r attribution.Result) string {
	a := r.Attribution
	s := r.Score
	return fmt.Sprintf("%s|%s|%s|%s|%s|%d|%v|%v",
		a.ProjectID, a.Role, a.Trade, a.WorkPackageID, a.AttemptID, a.Sequence, s.Cost, s.Overall)
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", joblearn.ErrInvalid, fmt.Sprintf(format, args...))
}
