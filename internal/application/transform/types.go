// Package transform is the client for iot-transform-fiware (/api/v0):
// transform rules (measurements and things-v2 events to NGSI-LD).
// Types mirror assets/docs/openapi.yaml + källkod in iot-transform-fiware at
// TRANSFORM identity policy (2026-10-01) plus PLAN1050 D/E-facit:
// CompatRequest{template,sources,rules,tenant,thingId,configRevision}/
// CompatResult{compatible,diagnostics} (POST /models/compat, register.go +
// compat_d_test.go) och preview {competing: []MatchDiagnostic,
// skipped: []Diagnostic, ruleId/ruleRevision/snapshotVersion,
// effectiveContext/removeAttributes} (preview.go + preview_e_test.go +
// RunPreviewTest). Match thingTypeVersion/variantId/variantVersion,
// målroller (Entity.Name), delta/snapshot-syntax (values/snapshot/delta +
// citerade vägar/index), removed[], previousToId och bootstrap-läsningar
// (export/relations) roundtrippar utan fältförlust via RawMessage.
// Hand-written; the source version is pinned in this comment so drift can
// be caught by comparing JSON keys against a checked-in OpenAPI copy.
package transform

import "encoding/json"

// Source describes where a property value comes from. Exactly one primary
// source must be set (backend validates).
type Source struct {
	Resource  string `json:"resource,omitempty"`
	Object    string `json:"object,omitempty"`
	Env       string `json:"env,omitempty"`
	Name      string `json:"name,omitempty"`
	Field     string `json:"field,omitempty"`
	Ref       string `json:"ref,omitempty"`
	Const     any    `json:"const,omitempty"`
	LatLon    bool   `json:"latlon,omitempty"`
	Timestamp bool   `json:"timestamp,omitempty"`
	Value     string `json:"value,omitempty"`
	Prefix    string `json:"prefix,omitempty"`
}

// Transform is a named single-value step.
type Transform struct {
	Op    string   `json:"op"`
	Value *float64 `json:"value,omitempty"`
}

// Derive is a named function over several sources.
type Derive struct {
	Fn string   `json:"fn"`
	Of []Source `json:"of"`
}

// Property describes one NGSI-LD attribute. Source fields are embedded
// flat (no inline tag: Go flattens embedded structs in JSON by default;
// backend uses yaml:",inline" for the same shape).
type Property struct {
	Array      bool        `json:"array,omitempty"`
	Target     string      `json:"target"`
	Type       string      `json:"type"`
	Unit       string      `json:"unit,omitempty"`
	Convert    *Conversion `json:"convert,omitempty"`
	ObservedAt string      `json:"observedAt,omitempty"`
	ObservedBy string      `json:"observedBy,omitempty"`
	When       string      `json:"when,omitempty"`
	Required   bool        `json:"required,omitempty"`
	Source
	Transform     []Transform `json:"transform,omitempty"`
	Derive        *Derive     `json:"derive,omitempty"`
	Subproperties []Property  `json:"subproperties,omitempty"`
}

type Conversion struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Factor *float64 `json:"factor,omitempty"`
	Offset float64  `json:"offset,omitempty"`
}

// Entity describes one NGSI-LD entity to write.
type Entity struct {
	Name             string     `json:"name,omitempty"`
	ID               string     `json:"id"`
	Type             string     `json:"type"`
	Context          string     `json:"context,omitempty"`
	Create           bool       `json:"create,omitempty"`
	Properties       []Property `json:"properties"`
	RemoveAttributes []string   `json:"removeAttributes,omitempty"`
	Delete           bool       `json:"delete,omitempty"`
}

// Match describes which sources a rule applies to. ThingTypeVersion,
// VariantID/VariantVersion pin exact template/variant versions (A-baseline
// additiv v1, omitempty so old backends ignore them).
type Match struct {
	Kind             string `json:"kind"`
	Device           string `json:"device,omitempty"`
	Port             string `json:"port,omitempty"`
	SensorType       string `json:"sensorType,omitempty"`
	Object           string `json:"object,omitempty"`
	Resource         string `json:"resource,omitempty"`
	Env              string `json:"env,omitempty"`
	Type             string `json:"type,omitempty"`
	SubType          string `json:"subType,omitempty"`
	ThingTypeVersion string `json:"thingTypeVersion,omitempty"`
	VariantID        string `json:"variantId,omitempty"`
	VariantVersion   string `json:"variantVersion,omitempty"`
	Tenant           string `json:"tenant,omitempty"`
	Event            string `json:"event,omitempty"`
	Relation         string `json:"relation,omitempty"`
	ToType           string `json:"toType,omitempty"`
	RelationRemoved  bool   `json:"relationRemoved,omitempty"`
	Lifecycle        string `json:"lifecycle,omitempty"`
}

// Rule matches a source and describes entities to write.
type Rule struct {
	Priority int `json:"priority,omitempty"`
	// Name är fritext endast för användaren. Påverkar aldrig matchning,
	// körning, prioritet eller specificitet.
	Name     string   `json:"name,omitempty"`
	Match    Match    `json:"match"`
	Entities []Entity `json:"entities"`
}

// Model is the API envelope for a rule: identity/lifecycle outside the
// rule (id never appears in YAML).
type Model struct {
	ID       string `json:"id"`
	Revision int64  `json:"revision"`
	Source   string `json:"source"`
	SeedKey  string `json:"seedKey,omitempty"`
	Kind     string `json:"kind"`
	Rule     Rule   `json:"rule"`
}

// PreviewEvent is the example event a preview runs against. Message is the
// raw envelope (thing.updated-, things.v1.values- or
// things.v1.relations-JSON for things, {pack,timestamp} for measurements)
// kept as RawMessage so no field is lost in roundtrip (single JSON doc).
type PreviewEvent struct {
	Kind       string          `json:"kind"`
	Message    json.RawMessage `json:"message"`
	SensorType string          `json:"sensorType,omitempty"`
}

// PreviewEntity is one previewed entity write. RemoveAttributes lists
// attributes cleared via PATCH-null (operation "remove"); EffectiveContext
// is the resolved JSON-LD context (backend preview.go, E-facit; empty means
// deployment default is used at write time). Context holds the explicit
// entity context, ContextSource is GUI-only (entity vs deployment default;
// backend preview never sends it, tolerated as empty). Operation is
// create/merge/remove/delete, same semantics as production writes.
type PreviewEntity struct {
	ID               string         `json:"id"`
	Type             string         `json:"type"`
	Properties       map[string]any `json:"properties,omitempty"`
	Context          string         `json:"context,omitempty"`
	EffectiveContext string         `json:"effectiveContext,omitempty"`
	ContextSource    string         `json:"contextSource,omitempty"`
	RemoveAttributes []string       `json:"removeAttributes,omitempty"`
	Operation        string         `json:"operation"`
}

// MatchDiagnostic describes ranking for one input (backend diagnostics.go,
// E-facit: preview competing[]). Winner marks the selected rule; others
// carry reason (lower priority / specificity / later snapshot order).
// Tenant-scoped: only rules visible to the caller are listed. JSON keys
// match backend exactly: index, ruleId, revision, snapshotVersion,
// priority, specificity, winner, reason.
type MatchDiagnostic struct {
	Index           int    `json:"index"`
	RuleID          string `json:"ruleId"`
	Revision        int64  `json:"revision"`
	SnapshotVersion int64  `json:"snapshotVersion"`
	Priority        int    `json:"priority"`
	Specificity     int    `json:"specificity"`
	Winner          bool   `json:"winner"`
	Reason          string `json:"reason"`
}

// RuleOverlap flags potentially overlapping matches (conservative: actual
// per-input shadowing is explained by MatchDiagnostic).
type RuleOverlap struct {
	First        int    `json:"first"`
	Second       int    `json:"second"`
	FirstRuleID  string `json:"firstRuleId"`
	SecondRuleID string `json:"secondRuleId"`
}

// PreviewMatch is the legacy GUI view of one competing match (derived from
// Competing; Index is dropped here, canonical Index lives in Competing).
// Kept so H/I handlers/components compiling against Matches keep working;
// new code should read Competing/Diagnostics.
type PreviewMatch struct {
	RuleID          string `json:"ruleId"`
	Revision        int64  `json:"revision,omitempty"`
	SnapshotVersion int64  `json:"snapshotVersion,omitempty"`
	Priority        int    `json:"priority,omitempty"`
	Specificity     int    `json:"specificity,omitempty"`
	Winner          bool   `json:"winner,omitempty"`
	Reason          string `json:"reason,omitempty"`
}

// PreviewSkipped is the legacy skip shape (reason/target). Deprecated:
// PreviewResult.Skipped is now []Diagnostic (E-facit, full Diagnostic
// fields). Kept as a type so old references still compile; use Diagnostic.
type PreviewSkipped struct {
	RuleID string `json:"ruleId,omitempty"`
	Reason string `json:"reason"`
	Entity string `json:"entity,omitempty"`
	Target string `json:"target,omitempty"`
}

// DiagnosticCode are the agreed diagnostic codes (D-facit diagnostics.go).
type DiagnosticCode string

const (
	CodeRequiredMissing  DiagnosticCode = "required-missing"
	CodeInvalidSource    DiagnosticCode = "invalid-source"
	CodeNoMatch          DiagnosticCode = "no-match"
	CodeAmbiguousMatch   DiagnosticCode = "ambiguous-match"
	CodeUnitConflict     DiagnosticCode = "unit-conflict"
	CodeGeometryInvalid  DiagnosticCode = "geometry-invalid"
	CodeNGSILDInvalid    DiagnosticCode = "ngsild-invalid"
	CodeRevisionConflict DiagnosticCode = "revision-conflict"
	CodeTenantDenied     DiagnosticCode = "tenant-denied"
	CodeTargetNotFound   DiagnosticCode = "target-notfound"
	CodeCardinality      DiagnosticCode = "cardinality"
	CodeDeletedTarget    DiagnosticCode = "deleted-target"
)

// Diagnostic is a structured rule/compatibility diagnosis (D/E-facit).
// Entity is the target role (Entity.Name) or "entity <n>" for anonymous
// entities; Attribute is the property target; Expression is the examined
// source path/template. Detail carries the human text. Legacy tolerant
// fields (Reason/Target/Message/Severity/TargetRole/TemplateID/Revision)
// preserve old G stub shapes (PreviewSkipped/CompatibilityIssue) without
// loss: old JSON with reason/target/message decodes here instead of being
// dropped, and re-encodes with both key sets when set.
type Diagnostic struct {
	Code            DiagnosticCode `json:"code,omitempty"`
	RuleID          string         `json:"ruleId,omitempty"`
	RuleRevision    int64          `json:"ruleRevision,omitempty"`
	SnapshotVersion int64          `json:"snapshotVersion,omitempty"`
	Entity          string         `json:"entity,omitempty"`
	Attribute       string         `json:"attribute,omitempty"`
	Expression      string         `json:"expression,omitempty"`
	Tenant          string         `json:"tenant,omitempty"`
	ThingID         string         `json:"thingId,omitempty"`
	ConfigRevision  int64          `json:"configRevision,omitempty"`
	Detail          string         `json:"detail,omitempty"`
	// Legacy tolerant aliases (old G shapes, omitempty so D wire is clean).
	Reason     string `json:"reason,omitempty"`
	Target     string `json:"target,omitempty"`
	Message    string `json:"message,omitempty"`
	Severity   string `json:"severity,omitempty"`
	TargetRole string `json:"targetRole,omitempty"`
	TemplateID string `json:"templateId,omitempty"`
	Revision   int64  `json:"revision,omitempty"`
}

// TemplateVersion is the explicit template underlay for compatibility
// (D-facit). Generic strings, no model names in engine: values compare
// exactly. Empty ThingTypeVersion/VariantID/VariantVersion is wildcard in
// matching (empty version matches all but rule must declare thingType).
type TemplateVersion struct {
	ThingType        string `json:"thingType"`
	SubType          string `json:"subType,omitempty"`
	ThingTypeVersion string `json:"thingTypeVersion,omitempty"`
	VariantID        string `json:"variantId,omitempty"`
	VariantVersion   string `json:"variantVersion,omitempty"`
}

// PropertyContract describes a known source property in values/snapshot/
// delta. Unit is the source unit when known (empty = unknown/dynamic, no
// conflict trial).
type PropertyContract struct {
	ID   string `json:"id"`
	Unit string `json:"unit,omitempty"`
}

// RelationSlot describes a named relation slot. AllowedTargets are allowed
// target types (empty = unknown/dynamic). Max 0 means unlimited.
type RelationSlot struct {
	Name           string   `json:"name"`
	AllowedTargets []string `json:"allowedTargets,omitempty"`
	Min            int      `json:"min,omitempty"`
	Max            int      `json:"max,omitempty"`
}

// SourceContract is the explicit source underlay. Empty/nil contract means
// unknown underlay: only statically impossible constructions are reported,
// dynamic/optional sources never give false complete guarantee.
type SourceContract struct {
	Properties []PropertyContract `json:"properties,omitempty"`
	Relations  []RelationSlot     `json:"relations,omitempty"`
	Geometry   bool               `json:"geometry,omitempty"`
	Location   bool               `json:"location,omitempty"`
	Metadata   []string           `json:"metadata,omitempty"`
}

// PreviewResult is the outcome of a dry run. Wire-facit is E (preview.go):
// matched, entities, observations, skippedReason, ruleId, ruleRevision,
// snapshotVersion, competing ([]MatchDiagnostic), skipped ([]Diagnostic).
// G keeps backend field "competing" verbatim and maps it to GUI views
// "matches"/"diagnostics" in the client (A-rekommendation Våg3); "skipped"
// is []Diagnostic with full fields (no loss). Legacy keys matches/
// diagnostics/overlaps + top-level effectiveContext/contextSource decode
// tolerantly (missing => zero) so old backends/stubs still work; they are
// populated from competing when absent.
type PreviewResult struct {
	Matched          bool              `json:"matched"`
	Entities         []PreviewEntity   `json:"entities"`
	Observations     int               `json:"observations"`
	SkippedReason    string            `json:"skippedReason,omitempty"`
	RuleID           string            `json:"ruleId,omitempty"`
	RuleRevision     int64             `json:"ruleRevision,omitempty"`
	SnapshotVersion  int64             `json:"snapshotVersion,omitempty"`
	EffectiveContext string            `json:"effectiveContext,omitempty"`
	ContextSource    string            `json:"contextSource,omitempty"`
	Matches          []PreviewMatch    `json:"matches,omitempty"`
	Skipped          []Diagnostic      `json:"skipped,omitempty"`
	Diagnostics      []MatchDiagnostic `json:"diagnostics,omitempty"`
	Overlaps         []RuleOverlap     `json:"overlaps,omitempty"`
	Competing        []MatchDiagnostic `json:"competing,omitempty"`
}

// CompatibilityIssue is the legacy GUI display shape (targetRole/message/
// severity). Kept so H/I templ/handlers compiling against Issues/Warnings
// keep working; canonical diagnostics live in CompatibilityResult.Diagnostics.
// New code should read Diagnostics; Issues is derived from it in the client.
type CompatibilityIssue struct {
	RuleID     string `json:"ruleId,omitempty"`
	TemplateID string `json:"templateId,omitempty"`
	Revision   int64  `json:"revision,omitempty"`
	TargetRole string `json:"targetRole,omitempty"`
	Attribute  string `json:"attribute,omitempty"`
	Expression string `json:"expression,omitempty"`
	Message    string `json:"message"`
	Severity   string `json:"severity,omitempty"`
}

// CompatibilityRequest is the offline/API input for compatibility: explicit
// template version + source contract against a candidate rule set (D-facit
// CompatRequest{template,sources,rules,tenant,thingId,configRevision} +
// POST /models/compat). Wire emits only D keys; legacy flat fields
// (TemplateID/TemplateVersion/VariantID/VariantVersion, json:"-") are kept
// so H/I handlers constructing the old shape still compile: MarshalJSON
// derives Template from them when Template is empty, UnmarshalJSON fills
// both views from either wire shape.
type CompatibilityRequest struct {
	Template       TemplateVersion `json:"template"`
	Sources        SourceContract  `json:"sources,omitempty"`
	Rules          []Rule          `json:"rules"`
	Tenant         string          `json:"tenant,omitempty"`
	ThingID        string          `json:"thingId,omitempty"`
	ConfigRevision int64           `json:"configRevision,omitempty"`
	// Legacy H/I construction aliases (never on wire, mapped to Template).
	TemplateID      string `json:"-"`
	TemplateVersion string `json:"-"`
	VariantID       string `json:"-"`
	VariantVersion  string `json:"-"`
}

// compatWire is the D wire shape for CompatibilityRequest.
type compatWire struct {
	Template       TemplateVersion `json:"template"`
	Sources        SourceContract  `json:"sources,omitempty"`
	Rules          []Rule          `json:"rules"`
	Tenant         string          `json:"tenant,omitempty"`
	ThingID        string          `json:"thingId,omitempty"`
	ConfigRevision int64           `json:"configRevision,omitempty"`
}

// compatLegacy is the old G flat shape (tolerant read only).
type compatLegacy struct {
	TemplateID      string           `json:"templateId,omitempty"`
	TemplateVersion string           `json:"templateVersion,omitempty"`
	VariantID       string           `json:"variantId,omitempty"`
	VariantVersion  string           `json:"variantVersion,omitempty"`
	Rules           []Rule           `json:"rules,omitempty"`
	Tenant          string           `json:"tenant,omitempty"`
	ThingID         string           `json:"thingId,omitempty"`
	ConfigRevision  int64            `json:"configRevision,omitempty"`
	Template        *TemplateVersion `json:"template,omitempty"`
	Sources         *SourceContract  `json:"sources,omitempty"`
}

// MarshalJSON emits D wire only, deriving Template from legacy flat fields
// when Template is empty (H/I backward compat).
func (r CompatibilityRequest) MarshalJSON() ([]byte, error) {
	tmpl := r.Template
	if tmpl.ThingType == "" && r.TemplateID != "" {
		tmpl.ThingType = r.TemplateID
	}
	if tmpl.ThingTypeVersion == "" && r.TemplateVersion != "" {
		tmpl.ThingTypeVersion = r.TemplateVersion
	}
	if tmpl.VariantID == "" && r.VariantID != "" {
		tmpl.VariantID = r.VariantID
	}
	if tmpl.VariantVersion == "" && r.VariantVersion != "" {
		tmpl.VariantVersion = r.VariantVersion
	}
	// SubType defaults from first rule when not set explicitly (rules carry
	// match.subType; compat template subType narrows it).
	if tmpl.SubType == "" && len(r.Rules) > 0 && r.Rules[0].Match.SubType != "" {
		// Do not force: only fill when template type matches rule type to
		// avoid cross-type leakage. Empty stays wildcard.
		if tmpl.ThingType == "" || tmpl.ThingType == r.Rules[0].Match.Type {
			tmpl.SubType = r.Rules[0].Match.SubType
		}
	}
	return json.Marshal(compatWire{
		Template: tmpl, Sources: r.Sources, Rules: r.Rules,
		Tenant: r.Tenant, ThingID: r.ThingID, ConfigRevision: r.ConfigRevision,
	})
}

// UnmarshalJSON accepts both D wire and old flat shape, filling both views.
func (r *CompatibilityRequest) UnmarshalJSON(data []byte) error {
	var wire compatWire
	wireErr := json.Unmarshal(data, &wire)
	var legacy compatLegacy
	legacyErr := json.Unmarshal(data, &legacy)
	if wireErr != nil && legacyErr != nil {
		return wireErr
	}
	// Prefer D wire when template object present.
	if wire.Template.ThingType != "" || len(wire.Rules) > 0 {
		r.Template = wire.Template
		r.Sources = wire.Sources
		r.Rules = wire.Rules
		r.Tenant = wire.Tenant
		r.ThingID = wire.ThingID
		r.ConfigRevision = wire.ConfigRevision
	} else {
		r.Template = TemplateVersion{}
		if legacy.Template != nil {
			r.Template = *legacy.Template
		}
		r.Sources = SourceContract{}
		if legacy.Sources != nil {
			r.Sources = *legacy.Sources
		}
		r.Rules = legacy.Rules
		r.Tenant = legacy.Tenant
		r.ThingID = legacy.ThingID
		r.ConfigRevision = legacy.ConfigRevision
	}
	// Always fill legacy flat view from template for H/I checks.
	r.TemplateID = r.Template.ThingType
	r.TemplateVersion = r.Template.ThingTypeVersion
	r.VariantID = r.Template.VariantID
	r.VariantVersion = r.Template.VariantVersion
	// Old flat keys win when D template empty but flat present.
	if r.Template.ThingType == "" && legacy.TemplateID != "" {
		r.Template.ThingType = legacy.TemplateID
		r.TemplateID = legacy.TemplateID
	}
	if r.Template.ThingTypeVersion == "" && legacy.TemplateVersion != "" {
		r.Template.ThingTypeVersion = legacy.TemplateVersion
		r.TemplateVersion = legacy.TemplateVersion
	}
	if r.Template.VariantID == "" && legacy.VariantID != "" {
		r.Template.VariantID = legacy.VariantID
		r.VariantID = legacy.VariantID
	}
	if r.Template.VariantVersion == "" && legacy.VariantVersion != "" {
		r.Template.VariantVersion = legacy.VariantVersion
		r.VariantVersion = legacy.VariantVersion
	}
	if len(r.Rules) == 0 && len(legacy.Rules) > 0 {
		r.Rules = legacy.Rules
	}
	return nil
}

// CompatibilityResult is the outcome (D-facit CompatResult{compatible,
// diagnostics} + legacy GUI views Issues/Warnings). Wire carries
// compatible+diagnostics; Issues/Warnings are derived in the client for
// H/I templ (which ranges over them) and decode tolerantly from old stubs.
type CompatibilityResult struct {
	Compatible  bool                 `json:"compatible"`
	Diagnostics []Diagnostic         `json:"diagnostics,omitempty"`
	Issues      []CompatibilityIssue `json:"issues,omitempty"`
	Warnings    []CompatibilityIssue `json:"warnings,omitempty"`
}

// DiagnosticToIssue maps a canonical Diagnostic to the legacy GUI display
// shape (no loss of displayed values; Code/Detail preserved in Message,
// Entity preserved in both Entity-flavoured TargetRole).
func DiagnosticToIssue(d Diagnostic) CompatibilityIssue {
	msg := d.Detail
	if msg == "" {
		msg = d.Message
	}
	if msg == "" {
		msg = d.Reason
	}
	if d.Code != "" && msg != "" {
		msg = string(d.Code) + ": " + msg
	} else if d.Code != "" {
		msg = string(d.Code)
	}
	if msg == "" {
		msg = "incompatible"
	}
	role := d.Entity
	if role == "" {
		role = d.TargetRole
	}
	if role == "" {
		role = d.Target
	}
	attr := d.Attribute
	if attr == "" && d.Target != "" && d.Entity != "" {
		// Old skipped target ("links") is an attribute-ish target; keep it
		// visible when canonical attribute is empty.
		attr = d.Target
	}
	expr := d.Expression
	if expr == "" {
		expr = d.Target
	}
	rev := d.RuleRevision
	if rev == 0 {
		rev = d.Revision
	}
	sev := d.Severity
	if sev == "" {
		sev = "error"
	}
	return CompatibilityIssue{
		RuleID: d.RuleID, TemplateID: d.TemplateID, Revision: rev,
		TargetRole: role, Attribute: attr, Expression: expr,
		Message: msg, Severity: sev,
	}
}

// IssueToDiagnostic maps a legacy issue back to canonical Diagnostic
// (best-effort; Message preserved in Detail, TargetRole in Entity).
func IssueToDiagnostic(i CompatibilityIssue) Diagnostic {
	detail := i.Message
	code := DiagnosticCode("")
	// Preserve "code: detail" prefix when derived above.
	for _, c := range []DiagnosticCode{
		CodeRequiredMissing, CodeInvalidSource, CodeNoMatch, CodeAmbiguousMatch,
		CodeUnitConflict, CodeGeometryInvalid, CodeNGSILDInvalid,
		CodeRevisionConflict, CodeTenantDenied, CodeTargetNotFound,
		CodeCardinality, CodeDeletedTarget,
	} {
		prefix := string(c) + ": "
		if len(detail) > len(prefix) && detail[:len(prefix)] == prefix {
			code = c
			detail = detail[len(prefix):]
			break
		}
		if detail == string(c) {
			code = c
			detail = ""
			break
		}
	}
	return Diagnostic{
		Code: code, RuleID: i.RuleID, RuleRevision: i.Revision,
		Entity: i.TargetRole, Attribute: i.Attribute, Expression: i.Expression,
		Detail: detail, Message: i.Message, Severity: i.Severity,
		TargetRole: i.TargetRole, TemplateID: i.TemplateID, Revision: i.Revision,
	}
}

// CompetingToMatches maps E competing[] to the legacy GUI Matches view
// (Index dropped; canonical Index stays in Competing).
func CompetingToMatches(in []MatchDiagnostic) []PreviewMatch {
	if len(in) == 0 {
		return []PreviewMatch{}
	}
	out := make([]PreviewMatch, 0, len(in))
	for _, m := range in {
		out = append(out, PreviewMatch{
			RuleID: m.RuleID, Revision: m.Revision,
			SnapshotVersion: m.SnapshotVersion, Priority: m.Priority,
			Specificity: m.Specificity, Winner: m.Winner, Reason: m.Reason,
		})
	}
	return out
}

// MatchesToCompeting maps legacy Matches back to canonical Competing
// (Index defaults to slice position when unknown).
func MatchesToCompeting(in []PreviewMatch) []MatchDiagnostic {
	if len(in) == 0 {
		return []MatchDiagnostic{}
	}
	out := make([]MatchDiagnostic, 0, len(in))
	for i, m := range in {
		out = append(out, MatchDiagnostic{
			Index: i, RuleID: m.RuleID, Revision: m.Revision,
			SnapshotVersion: m.SnapshotVersion, Priority: m.Priority,
			Specificity: m.Specificity, Winner: m.Winner, Reason: m.Reason,
		})
	}
	return out
}
