package similarity

import (
	"fmt"
	"sort"

	"github.com/greadee/aa/kernel/joblearn"
)

// Conflict is a pair of candidates that assert incompatible outcomes for the
// same subject. A and B are ordered canonically so each pair appears once.
type Conflict struct {
	A      joblearn.Candidate `json:"a"`
	B      joblearn.Candidate `json:"b"`
	Reason string             `json:"reason"`
}

// Conflicts returns conflicting candidate pairs in a canonical order. Two
// candidates conflict when they target the same subject and have opposite
// polarity: a positive pattern or strategy against a pitfall. Routing hints are
// neutral and never conflict.
func Conflicts(candidates []joblearn.Candidate) []Conflict {
	var out []Conflict
	seen := map[string]bool{}
	for i := 0; i < len(candidates); i++ {
		for j := i + 1; j < len(candidates); j++ {
			a, b := candidates[i], candidates[j]
			if !sameSubject(a, b) {
				continue
			}
			pa, pb := polarity(a.Kind), polarity(b.Kind)
			if pa == 0 || pb == 0 || pa != -pb {
				continue
			}
			a, b = orderPair(a, b)
			key := candidateKey(a) + "\x00" + candidateKey(b)
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, Conflict{
				A:      a,
				B:      b,
				Reason: fmt.Sprintf("%s and %s target the same subject", a.Kind, b.Kind),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if ai, aj := candidateKey(out[i].A), candidateKey(out[j].A); ai != aj {
			return ai < aj
		}
		return candidateKey(out[i].B) < candidateKey(out[j].B)
	})
	return out
}

func polarity(kind joblearn.CandidateKind) int {
	switch kind {
	case joblearn.CandidatePattern, joblearn.CandidateStrategy:
		return 1
	case joblearn.CandidatePitfall:
		return -1
	default:
		return 0
	}
}

func sameSubject(a, b joblearn.Candidate) bool {
	if a.Scope != "" && a.Scope == b.Scope {
		return true
	}
	return a.Level == b.Level && overlap(a.Applicability, b.Applicability)
}

func overlap(a, b *joblearn.Applicability) bool {
	if a == nil || b == nil {
		return false
	}
	return intersects(a.Roles, b.Roles) || intersects(a.Trades, b.Trades) || intersects(a.Languages, b.Languages)
}

func intersects(x, y []string) bool {
	if len(x) == 0 || len(y) == 0 {
		return false
	}
	set := make(map[string]bool, len(x))
	for _, v := range x {
		set[v] = true
	}
	for _, v := range y {
		if set[v] {
			return true
		}
	}
	return false
}

func orderPair(a, b joblearn.Candidate) (joblearn.Candidate, joblearn.Candidate) {
	if candidateKey(a) > candidateKey(b) {
		return b, a
	}
	return a, b
}

func candidateKey(c joblearn.Candidate) string {
	return string(c.Kind) + "|" + string(c.Level) + "|" + c.Scope + "|" + c.Title
}
