package pipeline

import (
	"testing"

	"github.com/everstacklabs/sentinel/internal/catalog"
	"github.com/everstacklabs/sentinel/internal/diff"
)

func TestAssessRisk_LargeChangeset(t *testing.T) {
	cs := &diff.ChangeSet{}
	// 26 new models → draft
	for i := 0; i < 26; i++ {
		cs.New = append(cs.New, diff.ModelChange{Name: "model"})
	}

	draft, blocked, _ := assessRisk(cs)
	if !draft {
		t.Error("expected draft for >25 changes")
	}
	if blocked {
		t.Error("should never block")
	}
}

func TestAssessRisk_ManyDeprecations(t *testing.T) {
	cs := &diff.ChangeSet{
		DeprecationCandidates: []diff.ModelChange{
			{Name: "a"}, {Name: "b"}, {Name: "c"}, {Name: "d"},
		},
	}

	draft, blocked, _ := assessRisk(cs)
	if !draft {
		t.Error("expected draft for >3 deprecation candidates")
	}
	if blocked {
		t.Error("should never block")
	}
}

func TestAssessRisk_NormalChangeset(t *testing.T) {
	cs := &diff.ChangeSet{
		New:     []diff.ModelChange{{Name: "a"}},
		Updated: []diff.ModelUpdate{{Name: "b"}},
	}

	draft, blocked, _ := assessRisk(cs)
	if draft {
		t.Error("expected non-draft for small changeset")
	}
	if blocked {
		t.Error("should never block")
	}
}

func TestAssessRisk_LargePriceDelta(t *testing.T) {
	cs := &diff.ChangeSet{
		Updated: []diff.ModelUpdate{
			{
				Name: "gpt-4o",
				Changes: []catalog.FieldChange{
					{Field: "cost.input_per_1k", OldValue: float64(0.005), NewValue: float64(0.01)},
				},
			},
		},
	}

	draft, _, _ := assessRisk(cs)
	if !draft {
		t.Error("expected draft for >35% price increase (100% increase)")
	}
}

func TestAssessRisk_SmallPriceDelta(t *testing.T) {
	cs := &diff.ChangeSet{
		Updated: []diff.ModelUpdate{
			{
				Name: "gpt-4o",
				Changes: []catalog.FieldChange{
					{Field: "cost.input_per_1k", OldValue: float64(0.005), NewValue: float64(0.006)},
				},
			},
		},
	}

	draft, _, _ := assessRisk(cs)
	if draft {
		t.Error("20% price increase should not trigger draft")
	}
}

func TestBumpSemver_NewModels(t *testing.T) {
	v, err := bumpSemver("2.1.3", true)
	if err != nil {
		t.Fatal(err)
	}
	if v != "2.2.0" {
		t.Errorf("expected 2.2.0, got %s", v)
	}
}

func TestBumpSemver_UpdatesOnly(t *testing.T) {
	v, err := bumpSemver("2.1.3", false)
	if err != nil {
		t.Fatal(err)
	}
	if v != "2.1.4" {
		t.Errorf("expected 2.1.4, got %s", v)
	}
}

func TestBumpSemver_InvalidVersion(t *testing.T) {
	_, err := bumpSemver("invalid", true)
	if err == nil {
		t.Error("expected error for invalid semver")
	}
}

func TestBumpSemver_ZeroVersion(t *testing.T) {
	v, err := bumpSemver("0.0.0", true)
	if err != nil {
		t.Fatal(err)
	}
	if v != "0.1.0" {
		t.Errorf("expected 0.1.0, got %s", v)
	}
}
