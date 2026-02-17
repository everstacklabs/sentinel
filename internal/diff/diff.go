package diff

import (
	"math"
	"strings"

	"github.com/everstacklabs/sentinel/internal/adapter"
	"github.com/everstacklabs/sentinel/internal/catalog"
)

// Compute compares discovered models against the existing catalog for a provider.
func Compute(provider string, discovered []adapter.DiscoveredModel, existing map[string]*catalog.Model) *ChangeSet {
	cs := &ChangeSet{Provider: provider}

	discoveredSet := make(map[string]bool, len(discovered))

	for _, d := range discovered {
		discoveredSet[d.Name] = true
		catalogModel := toCatalogModel(&d)

		existingModel, exists := existing[d.Name]
		if !exists {
			cs.New = append(cs.New, ModelChange{Name: d.Name, Model: catalogModel})
			continue
		}

		// Compare fields
		changes := computeFieldChanges(existingModel, catalogModel)
		if len(changes) > 0 {
			cs.Updated = append(cs.Updated, ModelUpdate{
				Name:    d.Name,
				Model:   catalogModel,
				Changes: changes,
			})
		} else {
			cs.Unchanged++
		}
	}

	// Find deprecation candidates: in catalog but not discovered.
	// Skip dated snapshots — they are filtered during discovery and
	// should not be flagged as deprecation candidates.
	var disappeared []ModelChange
	for name, model := range existing {
		if !discoveredSet[name] && !looksLikeDatedSnapshot(name) {
			disappeared = append(disappeared, ModelChange{Name: name, Model: model})
		}
	}

	// Try to match disappeared with new models (rename detection)
	cs.PossibleRenames = detectRenames(cs.New, disappeared)

	// Remaining disappeared models that weren't matched as renames
	renameNewNames := make(map[string]bool)
	renameOldNames := make(map[string]bool)
	for _, rp := range cs.PossibleRenames {
		renameNewNames[rp.NewName] = true
		renameOldNames[rp.OldName] = true
	}

	for _, mc := range disappeared {
		if !renameOldNames[mc.Name] {
			cs.DeprecationCandidates = append(cs.DeprecationCandidates, mc)
		}
	}

	return cs
}

func toCatalogModel(d *adapter.DiscoveredModel) *catalog.Model {
	m := &catalog.Model{
		Name:         d.Name,
		DisplayName:  d.DisplayName,
		Family:       d.Family,
		Status:       d.Status,
		Capabilities: d.Capabilities,
		Limits: catalog.Limits{
			MaxTokens:           d.Limits.MaxTokens,
			MaxCompletionTokens: d.Limits.MaxCompletionTokens,
		},
		Modalities: catalog.Modalities{
			Input:  d.Modalities.Input,
			Output: d.Modalities.Output,
		},
	}
	if d.Cost != nil {
		m.Cost = &catalog.Cost{
			InputPer1K:  d.Cost.InputPer1K,
			OutputPer1K: d.Cost.OutputPer1K,
		}
	}
	return m
}

func computeFieldChanges(existing, discovered *catalog.Model) []catalog.FieldChange {
	var changes []catalog.FieldChange

	// Skip display_name comparison for existing models — the catalog value
	// is authoritative; the inferred name is only a default for new models.

	if discovered.Family != "" && existing.Family != discovered.Family {
		changes = append(changes, catalog.FieldChange{Field: "family", OldValue: existing.Family, NewValue: discovered.Family})
	}
	if discovered.Status != "" && existing.Status != discovered.Status {
		changes = append(changes, catalog.FieldChange{Field: "status", OldValue: existing.Status, NewValue: discovered.Status})
	}
	if discovered.Cost != nil {
		if existing.Cost == nil {
			changes = append(changes, catalog.FieldChange{Field: "cost", OldValue: nil, NewValue: discovered.Cost})
		} else {
			if existing.Cost.InputPer1K != discovered.Cost.InputPer1K {
				changes = append(changes, catalog.FieldChange{Field: "cost.input_per_1k", OldValue: existing.Cost.InputPer1K, NewValue: discovered.Cost.InputPer1K})
			}
			if existing.Cost.OutputPer1K != discovered.Cost.OutputPer1K {
				changes = append(changes, catalog.FieldChange{Field: "cost.output_per_1k", OldValue: existing.Cost.OutputPer1K, NewValue: discovered.Cost.OutputPer1K})
			}
		}
	}
	if discovered.Limits.MaxTokens != 0 && existing.Limits.MaxTokens != discovered.Limits.MaxTokens {
		changes = append(changes, catalog.FieldChange{Field: "limits.max_tokens", OldValue: existing.Limits.MaxTokens, NewValue: discovered.Limits.MaxTokens})
	}
	if discovered.Limits.MaxCompletionTokens != 0 && existing.Limits.MaxCompletionTokens != discovered.Limits.MaxCompletionTokens {
		changes = append(changes, catalog.FieldChange{Field: "limits.max_completion_tokens", OldValue: existing.Limits.MaxCompletionTokens, NewValue: discovered.Limits.MaxCompletionTokens})
	}

	// Capabilities: check for new ones
	existingCaps := make(map[string]bool)
	for _, c := range existing.Capabilities {
		existingCaps[c] = true
	}
	for _, c := range discovered.Capabilities {
		if !existingCaps[c] {
			changes = append(changes, catalog.FieldChange{Field: "capabilities", OldValue: existing.Capabilities, NewValue: discovered.Capabilities})
			break
		}
	}

	return changes
}

// detectRenames finds potential renames by matching disappeared + new models
// with same family and similar limits/cost.
func detectRenames(newModels []ModelChange, disappeared []ModelChange) []RenamePair {
	var renames []RenamePair

	for _, newM := range newModels {
		for _, oldM := range disappeared {
			if newM.Model.Family != oldM.Model.Family || newM.Model.Family == "" {
				continue
			}

			// Check limits similarity (within 10%)
			if oldM.Model.Limits.MaxTokens > 0 && newM.Model.Limits.MaxTokens > 0 {
				ratio := float64(newM.Model.Limits.MaxTokens) / float64(oldM.Model.Limits.MaxTokens)
				if math.Abs(ratio-1.0) > 0.1 {
					continue
				}
			}

			// Check cost similarity (within 20%)
			if oldM.Model.Cost != nil && newM.Model.Cost != nil {
				if oldM.Model.Cost.InputPer1K > 0 {
					ratio := newM.Model.Cost.InputPer1K / oldM.Model.Cost.InputPer1K
					if math.Abs(ratio-1.0) > 0.2 {
						continue
					}
				}
			}

			renames = append(renames, RenamePair{
				OldName: oldM.Name,
				NewName: newM.Name,
				Reason:  "same family, similar limits/cost",
			})
		}
	}

	return renames
}

// looksLikeDatedSnapshot checks if a model name contains a date-like segment.
// Used to avoid flagging dated snapshots already in the catalog as deprecation candidates.
func looksLikeDatedSnapshot(name string) bool {
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return false
	}
	for _, p := range parts[1:] {
		if (len(p) == 4 || len(p) == 8) && isAllDigits(p) {
			return true
		}
	}
	// YYYY-MM-DD across three segments
	for i := 1; i+2 < len(parts); i++ {
		if len(parts[i]) == 4 && len(parts[i+1]) == 2 && len(parts[i+2]) == 2 &&
			isAllDigits(parts[i]) && isAllDigits(parts[i+1]) && isAllDigits(parts[i+2]) {
			return true
		}
	}
	return false
}

func isAllDigits(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}
