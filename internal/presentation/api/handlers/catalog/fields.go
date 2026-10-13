package catalog

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/diwise/diwise-web/internal/application/client"
	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	featurecatalog "github.com/diwise/diwise-web/internal/presentation/web/components/features/catalog"
)

func errorStatus(err error) int {
	if errors.Is(err, client.ErrUnauthorized) {
		return http.StatusUnauthorized
	}
	if errors.Is(err, client.ErrForbidden) {
		return http.StatusForbidden
	}
	return http.StatusOK
}

type formStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w formStatusWriter) WriteHeader(status int) {
	if status == http.StatusOK {
		status = w.status
	}
	w.ResponseWriter.WriteHeader(status)
}

func formatJSON(value any) string {
	b, _ := json.MarshalIndent(value, "", "  ")
	return string(b)
}

// Omitted fields preserve copied data (older forms); an explicitly empty field
// clears it. Reject unknown members rather than silently discard user input.
func decodeField(form map[string][]string, name string, dest any) error {
	if _, exists := form[name]; !exists {
		return nil
	}
	raw := firstForm(form, name)
	if raw == "" {
		raw = "null"
	}
	d := json.NewDecoder(strings.NewReader(raw))
	d.DisallowUnknownFields()
	fresh := reflect.New(reflect.ValueOf(dest).Elem().Type())
	if err := d.Decode(fresh.Interface()); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	var extra any
	if err := d.Decode(&extra); err != io.EOF {
		return fmt.Errorf("%s: expected one JSON document", name)
	}
	reflect.ValueOf(dest).Elem().Set(fresh.Elem())
	return nil
}

func templateFields(model *featurecatalog.TemplateNewViewModel, spec appthingsv2.TemplateSpec) {
	model.PrimaryProperty = spec.Template.PrimaryProperty
	model.RepeatGroups = strings.Join(spec.Template.RepeatGroups, "\n")
	model.AllowedQuantities = strings.Join(spec.Template.AllowedQuantities, "\n")
	model.Labels = strings.Join(spec.Template.Labels, "\n")
	model.PropertyDefs = formatJSON(spec.Template.PropertyDefs)
	model.Relations = formatJSON(spec.Template.Relations)
	model.Recipes = formatJSON(spec.Recipes)
	model.ParamInfo = formatJSON(spec.ParamInfo)
	model.OperatorVersions = formatJSON(spec.OperatorVersions)
	model.RelationsStructured = true
	for _, slot := range spec.Template.Relations {
		model.RelationRows = append(model.RelationRows, featurecatalog.RelationEditRow{
			Name: slot.Name, AllowedTargets: strings.Join(slot.AllowedTargets, "\n"), Min: strconv.Itoa(slot.Min), Max: strconv.Itoa(slot.Max),
		})
	}
	model.RelationRows = append(model.RelationRows, featurecatalog.RelationEditRow{})
}

func submittedTemplateFields(model *featurecatalog.TemplateNewViewModel, form map[string][]string) {
	model.PrimaryProperty = submittedRaw(form, "primaryProperty")
	model.RepeatGroups = submittedRaw(form, "repeatGroups")
	model.AllowedQuantities = submittedRaw(form, "allowedQuantities")
	model.Labels = submittedRaw(form, "labels")
	model.PropertyDefs = submittedRaw(form, "propertyDefs")
	model.Relations = submittedRaw(form, "relations")
	model.Recipes = submittedRaw(form, "recipes")
	model.ParamInfo = submittedRaw(form, "paramInfo")
	model.OperatorVersions = submittedRaw(form, "operatorVersions")
	model.RelationsStructured = firstForm(form, "relationRowsPresent") == "true"
	model.RelationRows = nil
	if model.RelationsStructured {
		for i, name := range form["relationName"] {
			model.RelationRows = append(model.RelationRows, featurecatalog.RelationEditRow{Name: name,
				AllowedTargets: rowValue(form, "relationTargets", i), Min: rowValue(form, "relationMin", i), Max: rowValue(form, "relationMax", i)})
		}
		if firstForm(form, "addRelation") == "true" {
			model.RelationRows = append(model.RelationRows, featurecatalog.RelationEditRow{})
		}
	}
}

func rowValue(form map[string][]string, key string, index int) string {
	if index < len(form[key]) {
		return form[key][index]
	}
	return ""
}

func parseRelationRows(form map[string][]string) ([]appthingsv2.RelationSpec, error) {
	result := []appthingsv2.RelationSpec{}
	for _, key := range []string{"relationTargets", "relationMin", "relationMax"} {
		if len(form[key]) != len(form["relationName"]) {
			return nil, fmt.Errorf("relations: incomplete row fields")
		}
	}
	for i, rawName := range form["relationName"] {
		name := strings.TrimSpace(rawName)
		if name == "" {
			continue
		} // Explicitly clearing the name removes the slot.
		parseCount := func(key string) (int, error) {
			raw := strings.TrimSpace(rowValue(form, key, i))
			if raw == "" {
				return 0, nil
			}
			n, err := strconv.Atoi(raw)
			if err != nil || n < 0 {
				return 0, fmt.Errorf("relations.%s.%s: expected a non-negative integer", name, key)
			}
			return n, nil
		}
		min, err := parseCount("relationMin")
		if err != nil {
			return nil, err
		}
		max, err := parseCount("relationMax")
		if err != nil {
			return nil, err
		}
		result = append(result, appthingsv2.RelationSpec{Name: name, AllowedTargets: parseLines(rowValue(form, "relationTargets", i)), Min: min, Max: max})
	}
	return result, nil
}

func submittedRaw(form map[string][]string, name string) string {
	if len(form[name]) > 0 {
		return form[name][0]
	}
	return ""
}
