package thingsv2

import (
	"context"
	"fmt"
	"slices"
	"strings"

	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	view "github.com/diwise/diwise-web/internal/presentation/web/components/features/thingsv2"
)

// Candidates are exported only from the selected tenant, then checked against
// the slot's actual target template. Backend validation remains authoritative.
func relationSlots(ctx context.Context, app thingsV2App, tenant, id string, slots []appthingsv2.RelationSpec, stored []appthingsv2.RelationRef, submitted map[string][]string) ([]view.RelationSlotViewModel, string) {
	if len(slots) == 0 {
		return nil, ""
	}
	candidates, err := app.ThingsV2().ExportThings(ctx, tenant)
	message := ""
	if err != nil {
		message = "Relationsmål kunde inte läsas: " + err.Error()
	}
	result := make([]view.RelationSlotViewModel, 0, len(slots))
	for _, slot := range slots {
		selected := []string{}
		if submitted != nil && submittedValue(submitted, "relationsPresent") == "true" {
			selected = submitted["relation."+slot.Name]
		} else {
			for _, relation := range stored {
				if relation.Name == slot.Name {
					selected = append(selected, relation.TargetThingID)
				}
			}
		}
		row := view.RelationSlotViewModel{Name: slot.Name, Min: slot.Min, Max: slot.Max}
		seen := map[string]bool{}
		for _, candidate := range candidates {
			if candidate.Tenant != "" && candidate.Tenant != tenant || slot.Name == "partOf" && candidate.ThingID == id {
				continue
			}
			if len(slot.AllowedTargets) > 0 && !slices.Contains(slot.AllowedTargets, candidate.TemplateID) {
				continue
			}
			label := candidate.Name
			if label == "" {
				label = candidate.ThingID
			}
			row.Options = append(row.Options, view.RelationOption{ID: candidate.ThingID, Label: label + " (" + candidate.TemplateID + ")", Selected: slices.Contains(selected, candidate.ThingID)})
			seen[candidate.ThingID] = true
		}
		// Keep missing/ineligible submitted targets visible and removable.
		for _, target := range selected {
			if !seen[target] {
				row.Options = append(row.Options, view.RelationOption{ID: target, Label: target + " — ej tillgängligt mål", Selected: true})
				seen[target] = true
			}
		}
		result = append(result, row)
	}
	return result, message
}

func parseRelations(ctx context.Context, app thingsV2App, tenant, id string, slots []appthingsv2.RelationSpec, stored []appthingsv2.RelationRef, form map[string][]string) ([]appthingsv2.RelationRef, string) {
	if submittedValue(form, "relationsPresent") != "true" {
		return stored, ""
	}
	result := []appthingsv2.RelationRef{}
	known := map[string]bool{}
	for _, slot := range slots {
		known[slot.Name] = true
		seen := map[string]bool{}
		for _, raw := range form["relation."+slot.Name] {
			targetID := strings.TrimSpace(raw)
			if targetID == "" {
				continue
			}
			if seen[targetID] {
				return nil, fmt.Sprintf("relation %s: duplicate target", slot.Name)
			}
			seen[targetID] = true
			if slot.Name == "partOf" && targetID == id {
				return nil, "partOf: cannot target itself"
			}
			target, err := app.ThingsV2().GetThing(ctx, tenant, targetID)
			if err != nil {
				return nil, fmt.Sprintf("relation %s: %s: %v", slot.Name, targetID, err)
			}
			if target.Tenant != "" && target.Tenant != tenant {
				return nil, "relation target is outside tenant"
			}
			if len(slot.AllowedTargets) > 0 && !slices.Contains(slot.AllowedTargets, target.TemplateID) {
				return nil, "relation " + slot.Name + ": target template not allowed"
			}
			result = append(result, appthingsv2.RelationRef{Name: slot.Name, TargetThingID: targetID, TargetTemplateID: target.TemplateID})
		}
		if len(seen) < slot.Min || slot.Max > 0 && len(seen) > slot.Max {
			return nil, fmt.Sprintf("relation %s: cardinality min=%d max=%d", slot.Name, slot.Min, slot.Max)
		}
	}
	for _, relation := range stored {
		if !known[relation.Name] {
			result = append(result, relation)
		}
	}
	return result, ""
}

func detailRelations(ctx context.Context, app thingsV2App, tenant, id string) ([]view.RelationViewModel, string) {
	result := []view.RelationViewModel{}
	message := ""
	resolved, err := app.ThingsV2().GetRelations(ctx, tenant, id)
	if err != nil {
		message = "Utgående relationer kunde inte läsas: " + err.Error()
	}
	for _, relation := range resolved {
		if relation.Target != nil && relation.Target.Tenant != "" && relation.Target.Tenant != tenant {
			message = "Backend returnerade relationsmål utanför vald tenant."
			continue
		}
		row := view.RelationViewModel{Name: relation.Name, ID: relation.TargetThingID, Label: relation.TargetThingID, Broken: relation.Target == nil}
		if relation.Target != nil && relation.Target.Name != "" {
			row.Label = relation.Target.Name
		}
		result = append(result, row)
	}
	// Reverse references are derived from the backend's tenant-scoped export;
	// never from a cross-tenant list or only the partOf hierarchy.
	objects, err := app.ThingsV2().ExportThings(ctx, tenant)
	if err != nil {
		return result, message + " Omvända relationer kunde inte läsas: " + err.Error()
	}
	for _, object := range objects {
		if object.Tenant != "" && object.Tenant != tenant {
			continue
		}
		for _, relation := range object.Relations {
			if relation.TargetThingID == id {
				label := object.Name
				if label == "" {
					label = object.ThingID
				}
				result = append(result, view.RelationViewModel{Name: relation.Name, ID: object.ThingID, Label: label, Incoming: true})
			}
		}
	}
	return result, message
}
