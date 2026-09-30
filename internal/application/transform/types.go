// Package transform is the client for iot-transform-fiware (/api/v0):
// transform rules (measurements and things-v2 events to NGSI-LD).
// Types mirror assets/docs/openapi.yaml in iot-transform-fiware at
// TRANSFORM identity policy (2026-10-01): Rule/Match/Entity/Property with
// lifecycle/delete/relations, Relationship array, LanguageMap and Geo-GeoJSON. Hand-written;
// the source version is pinned in this comment so drift can be caught by
// comparing JSON keys against a checked-in OpenAPI copy.
package transform

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

// Property describes one NGSI-LD attribute.
type Property struct {
	Array         bool   `json:"array,omitempty"`
	Target        string `json:"target"`
	Type          string `json:"type"`
	Unit          string `json:"unit,omitempty"`
	ObservedAt    string `json:"observedAt,omitempty"`
	ObservedBy    string `json:"observedBy,omitempty"`
	When          string `json:"when,omitempty"`
	Required      bool   `json:"required,omitempty"`
	Source        `json:",inline"`
	Transform     []Transform `json:"transform,omitempty"`
	Derive        *Derive     `json:"derive,omitempty"`
	Subproperties []Property  `json:"subproperties,omitempty"`
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

// Match describes which sources a rule applies to.
type Match struct {
	Kind            string `json:"kind"`
	Device          string `json:"device,omitempty"`
	Port            string `json:"port,omitempty"`
	SensorType      string `json:"sensorType,omitempty"`
	Object          string `json:"object,omitempty"`
	Resource        string `json:"resource,omitempty"`
	Env             string `json:"env,omitempty"`
	Type            string `json:"type,omitempty"`
	SubType         string `json:"subType,omitempty"`
	Tenant          string `json:"tenant,omitempty"`
	Event           string `json:"event,omitempty"`
	Relation        string `json:"relation,omitempty"`
	RelationRemoved bool   `json:"relationRemoved,omitempty"`
	Lifecycle       string `json:"lifecycle,omitempty"`
}

// Rule matches a source and describes entities to write.
type Rule struct {
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

// PreviewEvent is the example event a preview runs against.
type PreviewEvent struct {
	Kind       string `json:"kind"`
	Message    any    `json:"message"`
	SensorType string `json:"sensorType,omitempty"`
}

// PreviewEntity is one previewed entity write.
type PreviewEntity struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Context    string         `json:"context,omitempty"`
	Operation  string         `json:"operation"`
}

// PreviewResult is the outcome of a dry run.
type PreviewResult struct {
	Matched       bool            `json:"matched"`
	Entities      []PreviewEntity `json:"entities"`
	Observations  int             `json:"observations"`
	SkippedReason string          `json:"skippedReason,omitempty"`
}
