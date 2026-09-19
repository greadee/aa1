package similarity

import (
	"reflect"
	"testing"

	"github.com/greadee/aa/kernel/joblearn"
)

func cand(kind joblearn.CandidateKind, level joblearn.CandidateLevel, scope string, app *joblearn.Applicability) joblearn.Candidate {
	return joblearn.Candidate{
		Kind: kind, Level: level, Scope: scope,
		Title: string(kind) + " " + scope, Content: "c", Applicability: app, Confidence: 0.5,
	}
}

func TestConflictsDetectsOpposingCandidates(t *testing.T) {
	pRole := cand(joblearn.CandidatePattern, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})
	pitRole := cand(joblearn.CandidatePitfall, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})
	pArch := cand(joblearn.CandidatePattern, joblearn.LevelRole, "role:Architect", &joblearn.Applicability{Roles: []string{"Architect"}})
	strategy := cand(joblearn.CandidateStrategy, joblearn.LevelTask, "work_package:wp1", nil)
	costPit := cand(joblearn.CandidatePitfall, joblearn.LevelProject, "work_package:wp1", nil)
	route := cand(joblearn.CandidateRouting, joblearn.LevelWorkforce, "trade:backend", &joblearn.Applicability{Trades: []string{"backend"}})

	conflicts := Conflicts([]joblearn.Candidate{pRole, pitRole, pArch, strategy, costPit, route})
	if len(conflicts) != 2 {
		t.Fatalf("conflicts = %d, want 2: %+v", len(conflicts), conflicts)
	}
	if conflicts[0].A.Kind != joblearn.CandidatePattern || conflicts[0].B.Kind != joblearn.CandidatePitfall {
		t.Fatalf("conflict[0] = %+v", conflicts[0])
	}
	if conflicts[1].A.Kind != joblearn.CandidatePitfall || conflicts[1].B.Kind != joblearn.CandidateStrategy {
		t.Fatalf("conflict[1] = %+v", conflicts[1])
	}
	for _, c := range conflicts {
		if c.Reason == "" {
			t.Fatal("conflict is missing a reason")
		}
	}
}

func TestConflictsOrderIndependent(t *testing.T) {
	candidates := []joblearn.Candidate{
		cand(joblearn.CandidatePattern, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}}),
		cand(joblearn.CandidatePitfall, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}}),
		cand(joblearn.CandidatePitfall, joblearn.LevelProject, "work_package:wp1", nil),
		cand(joblearn.CandidateStrategy, joblearn.LevelTask, "work_package:wp1", nil),
	}
	first := Conflicts(candidates)
	shuffled := []joblearn.Candidate{candidates[3], candidates[0], candidates[2], candidates[1]}
	second := Conflicts(shuffled)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("conflicts depend on input order:\n%+v\n%+v", first, second)
	}
}

func TestConflictsRequireOpposingPolarityAndSubject(t *testing.T) {
	pattern := cand(joblearn.CandidatePattern, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})
	strategy := cand(joblearn.CandidateStrategy, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})
	pitArch := cand(joblearn.CandidatePitfall, joblearn.LevelRole, "role:Architect", &joblearn.Applicability{Roles: []string{"Architect"}})
	route := cand(joblearn.CandidateRouting, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})
	pitBuilder := cand(joblearn.CandidatePitfall, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})

	if got := Conflicts([]joblearn.Candidate{pattern, strategy}); len(got) != 0 {
		t.Fatalf("same polarity should not conflict: %+v", got)
	}
	if got := Conflicts([]joblearn.Candidate{pattern, pitArch}); len(got) != 0 {
		t.Fatalf("different subjects should not conflict: %+v", got)
	}
	if got := Conflicts([]joblearn.Candidate{route, pitBuilder, pattern, pattern}); len(got) != 1 {
		t.Fatalf("routing must be neutral and duplicates deduped: %+v", got)
	}
}

func TestConflictsDeduplicates(t *testing.T) {
	pit := cand(joblearn.CandidatePitfall, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})
	pattern := cand(joblearn.CandidatePattern, joblearn.LevelRole, "role:Builder", &joblearn.Applicability{Roles: []string{"Builder"}})
	if got := Conflicts([]joblearn.Candidate{pit, pattern, pit, pattern}); len(got) != 1 {
		t.Fatalf("duplicates produced %d conflicts, want 1", len(got))
	}
}
