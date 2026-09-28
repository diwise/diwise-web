// Package thingsv2 is the client for iot-things-v2 (/api/v1): things
// created from versioned templates, read per tenant (?tenant= is required
// for multi-tenant users; reads never fan out server-side).
package thingsv2

import (
	"encoding/json"
	"time"
)

// Location is a GeoJSON Point or Polygon (lon/lat, WGS84).
type Location struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
}

// Point returns lon/lat for Point locations.
func (l *Location) Point() (lon, lat float64, ok bool) {
	if l == nil || l.Type != "Point" {
		return 0, 0, false
	}
	var coords []float64
	if err := json.Unmarshal(l.Coordinates, &coords); err != nil || len(coords) < 2 {
		return 0, 0, false
	}
	return coords[0], coords[1], true
}

// PropertyValue is one current property value with unit and origin.
type PropertyValue struct {
	PropertyID  string    `json:"propertyId"`
	DisplayName string    `json:"displayName,omitempty"`
	Value       *float64  `json:"value,omitempty"`
	Quality     string    `json:"quality,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	ObservedAt  time.Time `json:"observedAt"`
	Source      string    `json:"source,omitempty"`
}

// Thing is a business object (sak): identity, tenant, template references,
// current values and the resolved primary value (GUI head data in list/map).
type Thing struct {
	ThingID         string                   `json:"thingId"`
	Tenant          string                   `json:"tenant"`
	Name            string                   `json:"name,omitempty"`
	Category        string                   `json:"category,omitempty"`
	Location        *Location                `json:"location,omitempty"`
	Metadata        map[string]string        `json:"metadata,omitempty"`
	TemplateID      string                   `json:"templateId"`
	TemplateVersion string                   `json:"templateVersion"`
	VariantID       string                   `json:"variantId,omitempty"`
	VariantVersion  string                   `json:"variantVersion,omitempty"`
	Revision        int64                    `json:"revision"`
	Values          map[string]PropertyValue `json:"values,omitempty"`
	Primary         *PropertyValue           `json:"primary,omitempty"`
}

// Template is a published template version (GUI: dropdowns + form building).
type Template struct {
	ID              string   `json:"id"`
	Version         string   `json:"version"`
	Category        string   `json:"category,omitempty"`
	DisplayName     string   `json:"displayName,omitempty"`
	Description     string   `json:"description,omitempty"`
	PrimaryProperty string   `json:"primaryProperty,omitempty"`
	Required        []string `json:"required,omitempty"`
	Optional        []string `json:"optional,omitempty"`
	// AllowedGeometries styr vilka GeoJSON-typer platsen får ha.
	// Tomt = punkt + polygon (serverns default).
	AllowedGeometries []string `json:"allowedGeometries,omitempty"`
	// AllowNoLocation tillåter saker utan plats.
	AllowNoLocation bool `json:"allowNoLocation,omitempty"`
	// RepeatGroups är egenskapsgrupper som får upprepas (punktprefix).
	RepeatGroups []string `json:"repeatGroups,omitempty"`
	// AllowedQuantities är tillåtna storheter för egenskaps-ID:n.
	AllowedQuantities []string `json:"allowedQuantities,omitempty"`
	// Labels är fritextetiketter för mallen.
	Labels       []string               `json:"labels,omitempty"`
	Relations    []RelationSpec         `json:"relations,omitempty"`
	PropertyDefs map[string]PropertyDef `json:"propertyDefs,omitempty"`
}

// PropertyDef describes a property: display name and compatible signals.
type PropertyDef struct {
	DisplayName string       `json:"displayName,omitempty"`
	Signals     []SignalHint `json:"signals,omitempty"`
}

// SignalHint is a compatible sensor signal (object URN + resource).
type SignalHint struct {
	Object   string `json:"object"`
	Resource string `json:"resource"`
}

// RelationSpec declares a relation slot: name, allowed target templates
// (empty = any), and min/max targets.
type RelationSpec struct {
	Name           string   `json:"name"`
	AllowedTargets []string `json:"allowedTargets,omitempty"`
	Min            int      `json:"min,omitempty"`
	Max            int      `json:"max,omitempty"`
}

// TemplateSpec wraps a template with parameters and recipes.
type TemplateSpec struct {
	Template      Template            `json:"template"`
	ParamDefaults map[string]float64  `json:"paramDefaults,omitempty"`
	Overridable   []string            `json:"overridable,omitempty"`
	ParamInfo     map[string]ParamDef `json:"paramInfo,omitempty"`
	Recipes       []Recipe            `json:"recipes,omitempty"`
}

// Recipe is a named calculation: operator version with inputs, outputs
// and params. GUI v1 visar recipes read-only och kopierar dem med vid
// ny version (full recipe-editor är deferred, PLAN002).
type Recipe struct {
	Name     string   `json:"name"`
	Operator string   `json:"operator"`
	Version  string   `json:"version,omitempty"`
	Inputs   []string `json:"inputs,omitempty"`
	Outputs  []string `json:"outputs,omitempty"`
	Params   []string `json:"params,omitempty"`
}

// ParamDef documents a parameter (unit, description, range).
type ParamDef struct {
	Unit        string   `json:"unit,omitempty"`
	Description string   `json:"description,omitempty"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
}

// Variant is a published variant version locked to a template version.
type Variant struct {
	ID              string             `json:"id"`
	Version         string             `json:"version"`
	TemplateID      string             `json:"templateId"`
	TemplateVersion string             `json:"templateVersion"`
	ParamValues     map[string]float64 `json:"paramValues,omitempty"`
}

// VariantSpec wraps a variant.
type VariantSpec struct {
	Variant Variant `json:"variant"`
}

// Filter selects things: exact matches plus case-insensitive name substring.
type Filter struct {
	Template        string
	TemplateVersion string
	Variant         string
	Name            string
	Category        string
	Limit           int
	Offset          int
}

// Result is one tenant's page: things sorted by thing ID with the total
// before paging.
type Result struct {
	Things []Thing
	Total  int
}

// HistoryPoint is one observed property value (oldest first).
type HistoryPoint struct {
	PropertyID string    `json:"propertyId"`
	ObservedAt time.Time `json:"observedAt"`
	Value      *float64  `json:"value,omitempty"`
	Quality    string    `json:"quality,omitempty"`
}

// Binding connects a device signal (device, object, resource) to a
// named calculation input. Empty channel matches all channels.
type Binding struct {
	DeviceID string `json:"deviceID"`
	Channel  string `json:"channel,omitempty"`
	Object   string `json:"object"`
	Resource string `json:"resource"`
	Input    string `json:"input"`
}

// Overview is a thing with its direct children (current values).
type Overview struct {
	Thing    Thing   `json:"thing"`
	Children []Thing `json:"children,omitempty"`
}

// Property is a thing-level property definition (presentation fields).
type Property struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName,omitempty"`
	DataType    string `json:"dataType,omitempty"`
	Quantity    string `json:"quantity,omitempty"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
}

// RelationRef points at another thing (partOf and friends).
type RelationRef struct {
	Name             string `json:"name"`
	TargetThingID    string `json:"targetThingId"`
	TargetTemplateID string `json:"targetTemplateId,omitempty"`
}

// ObjectSpec creates or replaces a thing. Template identity is fixed
// after creation: changing template means delete + recreate.
type ObjectSpec struct {
	ThingID         string              `json:"thingId"`
	Name            string              `json:"name"`
	Category        string              `json:"category,omitempty"`
	Location        *Location           `json:"location"`
	Metadata        map[string]string   `json:"metadata,omitempty"`
	TemplateID      string              `json:"templateId"`
	TemplateVersion string              `json:"templateVersion"`
	VariantID       string              `json:"variantId,omitempty"`
	VariantVersion  string              `json:"variantVersion,omitempty"`
	Properties      map[string]Property `json:"properties,omitempty"`
	Bindings        []Binding           `json:"bindings,omitempty"`
	ParamOverrides  map[string]float64  `json:"paramOverrides,omitempty"`
	Relations       []RelationRef       `json:"relations,omitempty"`
}

// EffectiveConfig is the stored materialized configuration: version
// references plus resolved values, origins and relations.
type EffectiveConfig struct {
	ThingID         string              `json:"thingId,omitempty"`
	Name            string              `json:"name,omitempty"`
	Category        string              `json:"category,omitempty"`
	Location        *Location           `json:"location,omitempty"`
	Metadata        map[string]string   `json:"metadata,omitempty"`
	TemplateID      string              `json:"templateId"`
	TemplateVersion string              `json:"templateVersion"`
	VariantID       string              `json:"variantId,omitempty"`
	VariantVersion  string              `json:"variantVersion,omitempty"`
	Properties      map[string]Property `json:"properties,omitempty"`
	Bindings        []Binding           `json:"bindings,omitempty"`
	ParamValues     map[string]float64  `json:"paramValues,omitempty"`
	ParamSources    map[string]string   `json:"paramSources,omitempty"`
	PrimaryProperty string              `json:"primaryProperty,omitempty"`
	Relations       []RelationRef       `json:"relations,omitempty"`
}
