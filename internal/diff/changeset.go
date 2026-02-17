package diff

import "github.com/everstacklabs/sentinel/internal/catalog"

// ChangeSet represents the complete diff between discovered and existing models.
type ChangeSet struct {
	Provider              string
	New                   []ModelChange
	Updated               []ModelUpdate
	DeprecationCandidates []ModelChange
	PossibleRenames       []RenamePair
	Unchanged             int
}

// ModelChange represents a new or deprecated model.
type ModelChange struct {
	Name  string
	Model *catalog.Model
}

// ModelUpdate represents an existing model with field changes.
type ModelUpdate struct {
	Name    string
	Model   *catalog.Model
	Changes []catalog.FieldChange
}

// RenamePair represents a possible rename (old model disappeared, new appeared).
type RenamePair struct {
	OldName string
	NewName string
	Reason  string // e.g., "same family, similar limits"
}

// HasChanges reports whether the changeset has any modifications.
func (cs *ChangeSet) HasChanges() bool {
	return len(cs.New) > 0 || len(cs.Updated) > 0 || len(cs.DeprecationCandidates) > 0
}

// TotalChanged returns the count of new + updated models.
func (cs *ChangeSet) TotalChanged() int {
	return len(cs.New) + len(cs.Updated)
}
