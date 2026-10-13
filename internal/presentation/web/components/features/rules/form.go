package rules

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	appthingsv2 "github.com/diwise/diwise-web/internal/application/thingsv2"
	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	. "github.com/diwise/frontend-toolkit"
)

// RuleFormViewModel preserves the candidate and submitted input separately.
// The complete JSON editor rejects unknown fields rather than dropping them.
type RuleFormViewModel struct {
	ID            string
	Revision      int64
	IsNew         bool
	CopyFrom      string
	IsSeed        bool
	ShowDelete    bool
	ConfirmDelete bool
	Tenants       []string
	Rule          apptransform.Rule
	// TypeOptions är mall-ID:n från katalogen (tomt = fritext + hint).
	TypeOptions []string
	// SensorTypeOptions är decoder-värden från device-profilerna.
	SensorTypeOptions []string
	// SubTypeOptions är subTyper från synliga syskonregler.
	SubTypeOptions      []string
	Notice              string
	ErrorMessage        string
	TransformConfigured bool
	Templates           []appthingsv2.TemplateSpec
	Variants            []appthingsv2.VariantSpec
	SourceError         string
	SubmittedForm       map[string][]string
}

// Heading är sidrubriken (lokaliserad via nyckel).
func (m RuleFormViewModel) Heading(l10n Localizer) string {
	if m.IsNew {
		return l10n.Get("rules_new")
	}
	return l10n.Get("rules_rule") + " " + ruleDisplayName(m.Rule.Name, m.ID)
}

// FormAction är formulärets POST-mål.
func (m RuleFormViewModel) FormAction() string {
	if m.IsNew {
		return "/rules/new"
	}
	return "/rules/" + m.ID
}

// RevisionString är revisionen som dolt fält (If-Match rev-n).
func (m RuleFormViewModel) RevisionString() string {
	return strconv.FormatInt(m.Revision, 10)
}

func (m RuleFormViewModel) PriorityString() string { return strconv.Itoa(m.Rule.Priority) }

func propertyConversionJSON(p apptransform.Property) string {
	if p.Convert == nil {
		return ""
	}
	raw, _ := json.Marshal(p.Convert)
	return string(raw)
}

func entityField(index int, name string) string {
	return fmt.Sprintf("e%d_%s", index, name)
}

func propertyField(entity, index int, name string) string {
	return fmt.Sprintf("e%d_p%d_%s", entity, index, name)
}

func propertyTarget(index int) string {
	return fmt.Sprintf("entity-%d-properties", index)
}

// propertyBlankVals bygger hx-vals för blanka property-block: entitet statiskt,
// index via JS-räknaren (ruleFormNext) så upprepade klick inte kolliderar.
func propertyBlankVals(entity int) string {
	return fmt.Sprintf(`js:{entity: %d, index: ruleFormNext('property', %d)}`, entity, entity)
}

func removeAttributesText(entity apptransform.Entity) string {
	var out strings.Builder
	for i, a := range entity.RemoveAttributes {
		if i > 0 {
			out.WriteString("\n")
		}
		out.WriteString(a)
	}
	return out.String()
}

func propertyTypes() []string {
	return []string{"Number", "Text", "Boolean", "DateTime", "Geo", "Relationship", "NumberArray", "TextArray", "Structured", "LanguageMap"}
}

func sourceKinds() []string {
	return []string{"resource", "object", "env", "name", "field", "ref", "const", "latlon", "timestamp", "value", "derive"}
}

// propertySourceKind väljer källväljaren efter satt källa (första
// träffen i källprioritetsordning).
func propertySourceKind(p apptransform.Property) string {
	switch {
	case p.Resource != "":
		return "resource"
	case p.Object != "":
		return "object"
	case p.Env != "":
		return "env"
	case p.Name != "":
		return "name"
	case p.Field != "":
		return "field"
	case p.Ref != "":
		return "ref"
	case p.Const != nil:
		return "const"
	case p.LatLon:
		return "latlon"
	case p.Timestamp:
		return "timestamp"
	case p.Value != "":
		return "value"
	default:
		if p.Derive != nil {
			return "derive"
		}
		return "resource"
	}
}

// propertySourceValue är källans textvärde (const: som JSON).
func propertySourceValue(p apptransform.Property) string {
	switch propertySourceKind(p) {
	case "resource":
		return p.Resource
	case "object":
		return p.Object
	case "env":
		return p.Env
	case "name":
		return p.Name
	case "field":
		return p.Field
	case "ref":
		return p.Ref
	case "const":
		raw, err := json.Marshal(p.Const)
		if err != nil {
			return ""
		}
		return string(raw)
	case "value":
		return p.Value
	default:
		return ""
	}
}

func jsonText(value any) string {
	raw, _ := json.MarshalIndent(value, "", "  ")
	return string(raw)
}

func (m RuleFormViewModel) RuleJSON() string {
	if values := m.SubmittedForm["ruleJSON"]; len(values) > 0 && m.EditorMode() == "json" {
		return values[0]
	}
	return jsonText(m.Rule)
}

func (m RuleFormViewModel) EditorMode() string {
	if values := m.SubmittedForm["editorMode"]; len(values) > 0 && values[0] == "json" {
		return "json"
	}
	return "form"
}

func (m RuleFormViewModel) SubmittedJSON() string { return jsonText(m.SubmittedForm) }

type EntityFormRow struct {
	Index  int
	Entity apptransform.Entity
}

type PropertyFormRow struct {
	Index    int
	Property apptransform.Property
}

var submittedRowPattern = regexp.MustCompile(`^e(\d+)_(?:p(\d+)_)?`)

func (m RuleFormViewModel) hasSubmittedRuleFields() bool {
	_, hasKind := m.SubmittedForm["kind"]
	return m.EditorMode() == "form" && hasKind
}

// On a parse failure Rule may contain only the prefix preceding the error.
// Render every submitted row, including sparse indexes, so the original
// controls can be restored rather than merely archived in an error message.
func (m RuleFormViewModel) EntityRows() []EntityFormRow {
	var rows []EntityFormRow
	if m.hasSubmittedRuleFields() {
		indices := map[int]bool{}
		for key := range m.SubmittedForm {
			match := submittedRowPattern.FindStringSubmatch(key)
			if match == nil {
				continue
			}
			i, err := strconv.Atoi(match[1])
			if err == nil && i >= 0 && i <= 99 {
				indices[i] = true
			}
		}
		for i := range indices {
			rows = append(rows, EntityFormRow{Index: i})
		}
		slices.SortFunc(rows, func(a, b EntityFormRow) int { return a.Index - b.Index })
		if len(rows) == len(m.Rule.Entities) {
			for i := range rows {
				rows[i].Entity = m.Rule.Entities[i]
			}
		}
		return rows
	}
	for i, entity := range m.Rule.Entities {
		rows = append(rows, EntityFormRow{i, entity})
	}
	return rows
}

func (m RuleFormViewModel) PropertyRows(entity int, properties []apptransform.Property) []PropertyFormRow {
	var rows []PropertyFormRow
	if m.hasSubmittedRuleFields() {
		indices := map[int]bool{}
		for key := range m.SubmittedForm {
			match := submittedRowPattern.FindStringSubmatch(key)
			if match == nil || match[2] == "" {
				continue
			}
			e, err := strconv.Atoi(match[1])
			if err != nil || e != entity {
				continue
			}
			p, err := strconv.Atoi(match[2])
			if err == nil && p >= 0 && p <= 99 {
				indices[p] = true
			}
		}
		for p := range indices {
			rows = append(rows, PropertyFormRow{Index: p})
		}
		slices.SortFunc(rows, func(a, b PropertyFormRow) int { return a.Index - b.Index })
		if len(rows) == len(properties) {
			for i := range rows {
				rows[i].Property = properties[i]
			}
		}
		return rows
	}
	for i, property := range properties {
		rows = append(rows, PropertyFormRow{i, property})
	}
	return rows
}

func (m RuleFormViewModel) SubmittedValue(key, fallback string) string {
	if values := m.SubmittedForm[key]; len(values) > 0 {
		return values[0]
	}
	return fallback
}

func (m RuleFormViewModel) TemplateVersions() []string {
	var versions []string
	for _, spec := range m.Templates {
		if spec.Template.ID == m.Rule.Match.Type {
			versions = append(versions, spec.Template.Version)
		}
	}
	return versions
}

func (m RuleFormViewModel) SelectedTemplate() *appthingsv2.TemplateSpec {
	for _, spec := range m.Templates {
		if spec.Template.ID == m.Rule.Match.Type && spec.Template.Version == m.Rule.Match.ThingTypeVersion {
			return &spec
		}
	}
	return nil
}

func (m RuleFormViewModel) MatchingVariants() []appthingsv2.VariantSpec {
	var variants []appthingsv2.VariantSpec
	for _, spec := range m.Variants {
		if spec.Variant.TemplateID == m.Rule.Match.Type && spec.Variant.TemplateVersion == m.Rule.Match.ThingTypeVersion {
			variants = append(variants, spec)
		}
	}
	return variants
}

func (m RuleFormViewModel) SelectedVariant() *appthingsv2.VariantSpec {
	for _, spec := range m.MatchingVariants() {
		if spec.Variant.ID == m.Rule.Match.VariantID && spec.Variant.Version == m.Rule.Match.VariantVersion {
			return &spec
		}
	}
	return nil
}

func valueSourcePaths(property string) []string {
	quoted, _ := json.Marshal(property)
	return []string{"delta." + string(quoted) + ".value", "snapshot." + string(quoted) + ".value"}
}

func propertyTransformOp(p apptransform.Property) string {
	if len(p.Transform) == 0 {
		return ""
	}
	return p.Transform[0].Op
}

func transformOps() []string {
	return []string{"", "multiply", "divide", "add", "floor", "round", "boolToOnOff", "numToOnOff", "safeURI"}
}

func propertyTransformValue(p apptransform.Property) string {
	if len(p.Transform) == 0 || p.Transform[0].Value == nil {
		return ""
	}
	return strconv.FormatFloat(*p.Transform[0].Value, 'f', -1, 64)
}

// propertyTransformJSON bevarar hela transform-kedjan (formuläret
// redigerar endast första steget; flerstegs bevaras orört).
func propertyTransformJSON(p apptransform.Property) string {
	if len(p.Transform) == 0 {
		return ""
	}
	raw, err := json.Marshal(p.Transform)
	if err != nil {
		return ""
	}
	return string(raw)
}

func propertyDeriveJSON(p apptransform.Property) string {
	if p.Derive == nil {
		return ""
	}
	raw, err := json.Marshal(p.Derive)
	if err != nil {
		return ""
	}
	return string(raw)
}

func propertySubpropertiesJSON(p apptransform.Property) string {
	if len(p.Subproperties) == 0 {
		return ""
	}
	raw, err := json.Marshal(p.Subproperties)
	if err != nil {
		return ""
	}
	return string(raw)
}

// previewFixtures är fixture-valen i preview-sektionen.
func previewFixtures() []Fixture {
	return Fixtures()
}

func previewKinds() []string {
	return []string{"measurement", "things.v1.values", "things.v1.relations", "things.v1.lifecycle"}
}

// defaultFixtureMessage är förifylld preview-text (första fixturen).
func defaultFixtureMessage() string {
	if all := Fixtures(); len(all) > 0 {
		return all[0].Message
	}
	return "{}"
}

// previewEmpty är otömd preview-modell vid sidladdning.
func previewEmpty() PreviewResultViewModel {
	return PreviewResultViewModel{}
}
