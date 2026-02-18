package diff

import (
	"testing"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/catalog"
)

func TestNewModelDetected(t *testing.T) {
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-5",
			DisplayName:  "GPT-5",
			Family:       "gpt-5",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.New) != 1 {
		t.Fatalf("expected 1 new model, got %d", len(cs.New))
	}
	if cs.New[0].Name != "gpt-5" {
		t.Errorf("expected new model gpt-5, got %s", cs.New[0].Name)
	}
	if cs.Unchanged != 0 {
		t.Errorf("expected 0 unchanged, got %d", cs.Unchanged)
	}
}

func TestUpdatedModelDetected(t *testing.T) {
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "beta",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.New) != 0 {
		t.Errorf("expected 0 new, got %d", len(cs.New))
	}
	if len(cs.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(cs.Updated))
	}
	if cs.Updated[0].Name != "gpt-4o" {
		t.Errorf("expected updated gpt-4o, got %s", cs.Updated[0].Name)
	}
	// Should have a status change
	found := false
	for _, c := range cs.Updated[0].Changes {
		if c.Field == "status" {
			found = true
		}
	}
	if !found {
		t.Error("expected status change")
	}
}

func TestDisplayNameChangeIgnored(t *testing.T) {
	// display_name differences should NOT be reported as changes for existing models
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT 4o Different",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4o",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.Updated) != 0 {
		t.Errorf("expected 0 updates (display_name-only changes ignored), got %d", len(cs.Updated))
	}
	if cs.Unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", cs.Unchanged)
	}
}

func TestDisplayNameChangeTracked(t *testing.T) {
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT 4o Different",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4o",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{TrackDisplayName: true})

	if len(cs.Updated) != 1 {
		t.Fatalf("expected 1 update with TrackDisplayName, got %d", len(cs.Updated))
	}
	found := false
	for _, c := range cs.Updated[0].Changes {
		if c.Field == "display_name" {
			found = true
		}
	}
	if !found {
		t.Error("expected display_name change when TrackDisplayName is true")
	}
}

func TestUnchangedModel(t *testing.T) {
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.New) != 0 || len(cs.Updated) != 0 {
		t.Errorf("expected no changes, got %d new, %d updated", len(cs.New), len(cs.Updated))
	}
	if cs.Unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", cs.Unchanged)
	}
	if cs.HasChanges() {
		t.Error("HasChanges() should be false for unchanged model")
	}
}

func TestDeprecationCandidate(t *testing.T) {
	discovered := []adapter.DiscoveredModel{} // nothing discovered
	existing := map[string]*catalog.Model{
		"gpt-old": {
			Name:         "gpt-old",
			DisplayName:  "GPT Old",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.DeprecationCandidates) != 1 {
		t.Fatalf("expected 1 deprecation candidate, got %d", len(cs.DeprecationCandidates))
	}
	if cs.DeprecationCandidates[0].Name != "gpt-old" {
		t.Errorf("expected gpt-old, got %s", cs.DeprecationCandidates[0].Name)
	}
}

func TestRenameDetection(t *testing.T) {
	// Same family, same limits → should detect rename
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o-v2",
			DisplayName:  "GPT-4O V2",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o-v1": {
			Name:         "gpt-4o-v1",
			DisplayName:  "GPT-4O V1",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000, MaxCompletionTokens: 16384},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.PossibleRenames) != 1 {
		t.Fatalf("expected 1 rename, got %d", len(cs.PossibleRenames))
	}
	if cs.PossibleRenames[0].OldName != "gpt-4o-v1" || cs.PossibleRenames[0].NewName != "gpt-4o-v2" {
		t.Errorf("expected rename gpt-4o-v1 → gpt-4o-v2, got %s → %s",
			cs.PossibleRenames[0].OldName, cs.PossibleRenames[0].NewName)
	}
	// Renamed models should NOT appear in deprecation candidates
	if len(cs.DeprecationCandidates) != 0 {
		t.Errorf("expected 0 deprecation candidates (rename matched), got %d", len(cs.DeprecationCandidates))
	}
}

func TestRenameMiss_DifferentFamily(t *testing.T) {
	// Different family → no rename, separate new + deprecation
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "embed-v2",
			DisplayName:  "Embed V2",
			Family:       "embedding",
			Status:       "stable",
			Capabilities: []string{"embeddings"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"embedding"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-old": {
			Name:         "gpt-old",
			DisplayName:  "GPT Old",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.PossibleRenames) != 0 {
		t.Errorf("expected 0 renames (different family), got %d", len(cs.PossibleRenames))
	}
	if len(cs.New) != 1 {
		t.Errorf("expected 1 new, got %d", len(cs.New))
	}
	if len(cs.DeprecationCandidates) != 1 {
		t.Errorf("expected 1 deprecation, got %d", len(cs.DeprecationCandidates))
	}
}

func TestDatedSnapshotNotDeprecated(t *testing.T) {
	// Dated snapshots in catalog should NOT appear as deprecation candidates
	discovered := []adapter.DiscoveredModel{}
	existing := map[string]*catalog.Model{
		"gpt-4o-2024-05-13": {
			Name:   "gpt-4o-2024-05-13",
			Family: "gpt-4",
			Status: "stable",
		},
		"gpt-5-2025-08-07": {
			Name:   "gpt-5-2025-08-07",
			Family: "gpt-5",
			Status: "stable",
		},
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4o",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	// Only gpt-4o (non-dated) should be a deprecation candidate
	if len(cs.DeprecationCandidates) != 1 {
		t.Fatalf("expected 1 deprecation candidate, got %d: %v", len(cs.DeprecationCandidates), cs.DeprecationCandidates)
	}
	if cs.DeprecationCandidates[0].Name != "gpt-4o" {
		t.Errorf("expected gpt-4o, got %s", cs.DeprecationCandidates[0].Name)
	}
}

func TestTotalChangedAndHasChanges(t *testing.T) {
	cs := &ChangeSet{
		New:     []ModelChange{{Name: "a"}, {Name: "b"}},
		Updated: []ModelUpdate{{Name: "c"}},
	}
	if cs.TotalChanged() != 3 {
		t.Errorf("TotalChanged() = %d, want 3", cs.TotalChanged())
	}
	if !cs.HasChanges() {
		t.Error("HasChanges() should be true")
	}
}

func TestCostChangeDetection(t *testing.T) {
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
			Cost:         &adapter.Cost{InputPer1K: 0.01, OutputPer1K: 0.03},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
			Cost:         &catalog.Cost{InputPer1K: 0.005, OutputPer1K: 0.015},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.Updated) != 1 {
		t.Fatalf("expected 1 updated, got %d", len(cs.Updated))
	}
	costFields := 0
	for _, c := range cs.Updated[0].Changes {
		if c.Field == "cost.input_per_1k" || c.Field == "cost.output_per_1k" {
			costFields++
		}
	}
	if costFields != 2 {
		t.Errorf("expected 2 cost field changes, got %d", costFields)
	}
}

func TestZeroCostIgnored(t *testing.T) {
	// Discovered model with zero cost should not overwrite existing cost data
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
			Cost:         &adapter.Cost{InputPer1K: 0, OutputPer1K: 0},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
			Cost:         &catalog.Cost{InputPer1K: 0.005, OutputPer1K: 0.015},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.Updated) != 0 {
		t.Errorf("expected 0 updates (zero-cost treated as missing data), got %d", len(cs.Updated))
	}
	if cs.Unchanged != 1 {
		t.Errorf("expected 1 unchanged, got %d", cs.Unchanged)
	}
}

func TestCapabilityRemovalDetected(t *testing.T) {
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat", "vision"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.Updated) != 1 {
		t.Fatalf("expected 1 updated (capability removed), got %d", len(cs.Updated))
	}
	found := false
	for _, c := range cs.Updated[0].Changes {
		if c.Field == "capabilities" {
			found = true
		}
	}
	if !found {
		t.Error("expected capabilities change for removal")
	}
}

func TestModalityChangeDetected(t *testing.T) {
	discovered := []adapter.DiscoveredModel{
		{
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       adapter.Limits{MaxTokens: 128000},
			Modalities:   adapter.Modalities{Input: []string{"text", "image", "audio"}, Output: []string{"text"}},
		},
	}
	existing := map[string]*catalog.Model{
		"gpt-4o": {
			Name:         "gpt-4o",
			DisplayName:  "GPT-4O",
			Family:       "gpt-4",
			Status:       "stable",
			Capabilities: []string{"chat"},
			Limits:       catalog.Limits{MaxTokens: 128000},
			Modalities:   catalog.Modalities{Input: []string{"text", "image"}, Output: []string{"text"}},
		},
	}

	cs := Compute("openai", discovered, existing, DiffOptions{})

	if len(cs.Updated) != 1 {
		t.Fatalf("expected 1 updated (modality change), got %d", len(cs.Updated))
	}
	found := false
	for _, c := range cs.Updated[0].Changes {
		if c.Field == "modalities.input" {
			found = true
		}
	}
	if !found {
		t.Error("expected modalities.input change")
	}
}
