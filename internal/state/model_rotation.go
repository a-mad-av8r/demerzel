package state

import (
	"slices"
	"strings"
)

// GroupModelKey limits rotation progress to a same-named model in one Group; credentials in the Group share progress.
type GroupModelKey struct {
	GroupID       uint
	ExternalModel string
}

// SelectModel receives available models ordered by upstream ID; it consumes only model rotation and does not change credential shares.
func (s *SchedulingState) SelectModel(groupID uint, externalModel string, models []string) string {
	if len(models) == 0 {
		return ""
	}
	selected := models[0]
	s.WithLock(func(ledger *SchedulingLedger) {
		key := GroupModelKey{GroupID: groupID, ExternalModel: externalModel}
		order := ledger.ModelCursors[key]
		for index, model := range order {
			if _, available := slices.BinarySearch(models, model); !available {
				continue
			}
			selected = model
			// Move only the selected model to the end, preserving the order of models still available to other credentials.
			copy(order[index:], order[index+1:])
			order[len(order)-1] = model
			break
		}
	})
	return selected
}

func (s *SchedulingState) syncModelCursorsLocked(snapshot *ConfigSnapshot) {
	cursors := make(map[GroupModelKey][]string)
	for key, order := range s.ledger.ModelCursors {
		if group, exists := snapshot.GroupCatalog[key.GroupID]; exists && !group.Enabled {
			cursors[key] = order
		}
	}
	for groupID, group := range snapshot.Groups {
		modelsByName := make(map[string][]string)
		for _, model := range group.Models {
			name := externalModelName(model)
			modelsByName[name] = append(modelsByName[name], strings.TrimSpace(model.ID))
		}
		for name, models := range modelsByName {
			if len(models) > 1 {
				slices.Sort(models)
				key := GroupModelKey{GroupID: groupID, ExternalModel: name}
				cursors[key] = reconcileModelOrder(s.ledger.ModelCursors[key], models)
			}
		}
	}
	s.ledger.ModelCursors = cursors
}

// Configuration publication and checkpoint recovery retain rotation order for extant models, appending new models to the end.
func reconcileModelOrder(previous, configured []string) []string {
	remaining := make(map[string]struct{}, len(configured))
	for _, model := range configured {
		remaining[model] = struct{}{}
	}
	order := make([]string, 0, len(configured))
	for _, model := range previous {
		if _, exists := remaining[model]; exists {
			order = append(order, model)
			delete(remaining, model)
		}
	}
	for _, model := range configured {
		if _, exists := remaining[model]; exists {
			order = append(order, model)
			delete(remaining, model)
		}
	}
	return order
}
