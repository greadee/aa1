package similarity

import (
	"errors"
	"reflect"
	"testing"

	"github.com/greadee/aa/kernel/joblearn"
	"github.com/greadee/aa/kernel/joblearn/attribution"
)

func result(role, trade string, outcome joblearn.Outcome, cost, duration float64, attempt string) attribution.Result {
	return attribution.Result{
		Attribution: joblearn.Attribution{
			Role: role, Trade: trade, Outcome: outcome, AttemptID: attempt, WorkPackageID: "wp-" + attempt,
		},
		Score: joblearn.Score{Version: joblearn.MetricVersion, Cost: cost, Duration: duration, Overall: 0.5},
	}
}

func TestFeatures(t *testing.T) {
	got := Features(result("Builder", "backend", joblearn.OutcomeSucceeded, 0.1, 0.8, "a1"))
	want := map[string]bool{
		"role:Builder": true, "trade:backend": true, "outcome:succeeded": true,
		"cost:low": true, "duration:high": true,
	}
	if len(got) != len(want) {
		t.Fatalf("features = %v, want %v", got, want)
	}
	for _, f := range got {
		if !want[f] {
			t.Fatalf("unexpected feature %q in %v", f, got)
		}
	}
	if len(Features(result("", "", joblearn.OutcomeSucceeded, 0.5, 0.5, "a2"))) != 3 {
		t.Fatal("empty identifiers should be omitted")
	}
}

func TestSimilarity(t *testing.T) {
	a := result("Builder", "backend", joblearn.OutcomeSucceeded, 0.1, 0.2, "a1")
	if got := Similarity(a, a); got != 1 {
		t.Fatalf("self similarity = %v, want 1", got)
	}
	b := result("Architect", "qa", joblearn.OutcomeFailed, 0.9, 0.9, "a2")
	if got := Similarity(a, b); got != 0 {
		t.Fatalf("disjoint similarity = %v, want 0", got)
	}
}

func TestClusterGroups(t *testing.T) {
	a := result("Builder", "backend", joblearn.OutcomeSucceeded, 0.1, 0.1, "a1")
	b := result("Builder", "backend", joblearn.OutcomeSucceeded, 0.2, 0.7, "a2")
	c := result("Builder", "qa", joblearn.OutcomeFailed, 0.9, 0.9, "a3")

	clusters, err := Cluster([]attribution.Result{a, b, c}, DefaultThreshold)
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	if len(clusters) != 2 {
		t.Fatalf("clusters = %d, want 2: %+v", len(clusters), clusters)
	}
	if len(clusters[0].Results) != 2 || len(clusters[1].Results) != 1 {
		t.Fatalf("cluster sizes = %d/%d", len(clusters[0].Results), len(clusters[1].Results))
	}
	if clusters[1].Results[0].Attribution.AttemptID != "a3" {
		t.Fatalf("outlier = %+v", clusters[1].Results[0].Attribution)
	}

	shuffled, err := Cluster([]attribution.Result{c, a, b}, DefaultThreshold)
	if err != nil {
		t.Fatalf("Cluster: %v", err)
	}
	if !reflect.DeepEqual(clusters, shuffled) {
		t.Fatalf("clustering depends on input order:\n%+v\n%+v", clusters, shuffled)
	}
}

func TestClusterEmptyAndValidation(t *testing.T) {
	clusters, err := Cluster(nil, DefaultThreshold)
	if err != nil || clusters != nil {
		t.Fatalf("clusters = %+v, err = %v", clusters, err)
	}
	if _, err := Cluster(nil, -0.1); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
	if _, err := Cluster(nil, 1.1); !errors.Is(err, joblearn.ErrInvalid) {
		t.Fatalf("err = %v, want ErrInvalid", err)
	}
}
