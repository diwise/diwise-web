package rules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	apptransform "github.com/diwise/diwise-web/internal/application/transform"
	. "github.com/diwise/frontend-toolkit"
)

// RuleFormViewModel är ny/skapa/redigera-formuläret för regler. Komplexa
// delar utan formulärstöd (derive, subproperties, flerstegs-transform)
// bevaras via dolda JSON-fält per property (B5: ingen YAML-visning).
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
}

// Heading är sidrubriken (lokaliserad via nyckel).
func (m RuleFormViewModel) Heading(l10n Localizer) string {
	if m.IsNew {
		return l10n.Get("rules_new")
	}
	return l10n.Get("rules_rule") + " " + m.ID
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
	return []string{"resource", "object", "env", "name", "field", "ref", "const", "latlon", "timestamp", "value"}
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
		if s, ok := p.Const.(string); ok {
			return s
		}
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
