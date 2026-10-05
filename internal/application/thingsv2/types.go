// Package thingsv2 is the client for iot-things-v2 (/api/v1): things
// created from versioned templates, read per tenant (?tenant= is required
// for multi-tenant users; reads never fan out server-side).
package thingsv2

import (
	"encoding/json"
	"strconv"
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
// AlternativeName/SubType follow K1 (NameOrID = alternativeName > name >
// thingId; TypeName = subType > thingType). ParamValues/ParamSources mirror
// the stored effective config so list/detail views need no extra join.
type Thing struct {
	ThingID         string                   `json:"thingId"`
	Tenant          string                   `json:"tenant"`
	Name            string                   `json:"name,omitempty"`
	AlternativeName string                   `json:"alternativeName,omitempty"`
	SubType         string                   `json:"subType,omitempty"`
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
	ParamValues     map[string]float64       `json:"paramValues,omitempty"`
	ParamSources    map[string]string        `json:"paramSources,omitempty"`
}

// thingWire is the wire shape for Thing with A-baseline delta alias:
// producers may emit "delta" instead of (or alongside) "values".
// Canonical writes use "values"; reads accept either (values wins).
type thingWire struct {
	ThingID         string                   `json:"thingId"`
	Tenant          string                   `json:"tenant"`
	Name            string                   `json:"name,omitempty"`
	AlternativeName string                   `json:"alternativeName,omitempty"`
	SubType         string                   `json:"subType,omitempty"`
	Category        string                   `json:"category,omitempty"`
	Location        *Location                `json:"location,omitempty"`
	Metadata        map[string]string        `json:"metadata,omitempty"`
	TemplateID      string                   `json:"templateId"`
	TemplateVersion string                   `json:"templateVersion"`
	VariantID       string                   `json:"variantId,omitempty"`
	VariantVersion  string                   `json:"variantVersion,omitempty"`
	Revision        int64                    `json:"revision"`
	Values          map[string]PropertyValue `json:"values,omitempty"`
	Delta           map[string]PropertyValue `json:"delta,omitempty"`
	Primary         *PropertyValue           `json:"primary,omitempty"`
	ParamValues     map[string]float64       `json:"paramValues,omitempty"`
	ParamSources    map[string]string        `json:"paramSources,omitempty"`
}

// UnmarshalJSON accepts both "values" and A-baseline "delta" (values wins).
func (t *Thing) UnmarshalJSON(data []byte) error {
	var w thingWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	t.ThingID = w.ThingID
	t.Tenant = w.Tenant
	t.Name = w.Name
	t.AlternativeName = w.AlternativeName
	t.SubType = w.SubType
	t.Category = w.Category
	t.Location = w.Location
	t.Metadata = w.Metadata
	t.TemplateID = w.TemplateID
	t.TemplateVersion = w.TemplateVersion
	t.VariantID = w.VariantID
	t.VariantVersion = w.VariantVersion
	t.Revision = w.Revision
	t.Values = w.Values
	if t.Values == nil {
		t.Values = w.Delta
	}
	t.Primary = w.Primary
	t.ParamValues = w.ParamValues
	t.ParamSources = w.ParamSources
	return nil
}

// MarshalJSON writes canonical "values" (no duplicate "delta").
func (t Thing) MarshalJSON() ([]byte, error) {
	type plain Thing
	return json.Marshal(struct {
		plain
		Delta map[string]PropertyValue `json:"delta,omitempty"`
	}{
		plain: plain(t),
		Delta: nil,
	})
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

// PropertyDef describes one allowed property for GUI form building:
// display name, documentation, data type, quantity/unit defaults and
// compatible sensor signals. Mirrors iot-things-v2 things.PropertyDef
// (backend is facit; unknown fields are ignored on decode).
type PropertyDef struct {
	DisplayName string       `json:"displayName,omitempty"`
	Description string       `json:"description,omitempty"`
	DataType    string       `json:"dataType,omitempty"`
	Quantity    string       `json:"quantity,omitempty"`
	Unit        string       `json:"unit,omitempty"`
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
	// OperatorVersions pins operator versions per operator name
	// (backend key "operatorVersions"; additive, omitempty).
	OperatorVersions map[string]string `json:"operatorVersions,omitempty"`
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
	// PresetProperties lists preset property IDs allowed by the template
	// (backend key "presetProperties"; additive, omitempty).
	PresetProperties []string `json:"presetProperties,omitempty"`
}

// BBox is a WGS84 bounding box for location search. Encoded as
// "minLon,minLat,maxLon,maxLat" in ?bbox= (backend parseBBox).
type BBox struct {
	MinLon, MinLat, MaxLon, MaxLat float64
}

// NearFilter selects things within RadiusMeters of Lon/Lat. Encoded as
// "lon,lat,radiusMeters" in ?near= (backend parseNear).
type NearFilter struct {
	Lon, Lat     float64
	RadiusMeters float64
}

// Filter selects things: exact matches plus case-insensitive name substring,
// optional location filters and paging. BBox/Near mirror backend
// SearchFilter; Metadata supports ?meta.<key>= filters.
type Filter struct {
	Template        string
	TemplateVersion string
	Variant         string
	VariantVersion  string
	Name            string
	Category        string
	Tag             string
	Metadata        map[string]string
	BBox            *BBox
	Near            *NearFilter
	Limit           int
	Offset          int
}

// Result is one tenant's page: things sorted by thing ID with the total
// before paging.
type Result struct {
	Things []Thing
	Total  int
}

// HistoryPoint is one observed property value (oldest first). ReceivedAt,
// Source, MessageID and Late mirror backend HistoryWrite: Late marks late
// observations stored without becoming current (B13); invalid observations
// are stored without Late (denied by quality, not time). Unit is the frozen
// source unit for the observation (omitempty, never relabeled without
// convert); missing unit means unknown/dynamic, not zero.
type HistoryPoint struct {
	PropertyID string    `json:"propertyId"`
	ObservedAt time.Time `json:"observedAt"`
	ReceivedAt time.Time `json:"receivedAt"`
	Value      *float64  `json:"value,omitempty"`
	Quality    string    `json:"quality,omitempty"`
	Unit       string    `json:"unit,omitempty"`
	Source     string    `json:"source,omitempty"`
	MessageID  string    `json:"messageId,omitempty"`
	Late       bool      `json:"late,omitempty"`
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

// RelationRef points at another thing (generic named slots; partOf is one
// slot among many with its own hierarchy rules).
type RelationRef struct {
	Name             string `json:"name"`
	TargetThingID    string `json:"targetThingId"`
	TargetTemplateID string `json:"targetTemplateId,omitempty"`
}

// RelationResolve is a named relation with its resolved target. Target is
// nil when the target is missing (deleted after linking) — the row is kept
// so broken references stay visible instead of silently vanishing.
// Mirrors backend application.RelationResolve.
type RelationResolve struct {
	Name          string `json:"name"`
	TargetThingID string `json:"targetThingId"`
	Target        *Thing `json:"target,omitempty"`
}

// TreeNode is a node in a subtree: the thing plus nested children.
// Mirrors backend application.TreeNode.
type TreeNode struct {
	Thing    Thing      `json:"thing"`
	Children []TreeNode `json:"children,omitempty"`
}

// Diff describes template/variant changes for upgrade preview. Mirrors
// backend catalog.Diff.
type Diff struct {
	AddedProps     []string `json:"addedProperties,omitempty"`
	RemovedProps   []string `json:"removedProperties,omitempty"`
	AddedParams    []string `json:"addedParams,omitempty"`
	RemovedParams  []string `json:"removedParams,omitempty"`
	ChangedRecipes []string `json:"changedRecipes,omitempty"`
}

// UpgradeResult is the outcome of preview/upgrade: diff plus conflicts that
// must be resolved explicitly (never dropped silently).
type UpgradeResult struct {
	Diff      Diff     `json:"diff"`
	Conflicts []string `json:"conflicts"`
}

// ObjectSpec creates or replaces a thing. Template identity is fixed
// after creation: changing template means delete + recreate (or Upgrade).
// Tenant is carried for roundtrip but never widens token auth: the query
// ?tenant= (auth-resolved) wins; backend overwrites body tenant.
type ObjectSpec struct {
	ThingID         string              `json:"thingId"`
	Tenant          string              `json:"tenant,omitempty"`
	Name            string              `json:"name,omitempty"`
	AlternativeName string              `json:"alternativeName,omitempty"`
	SubType         string              `json:"subType,omitempty"`
	Category        string              `json:"category,omitempty"`
	Location        *Location           `json:"location,omitempty"`
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
// references plus resolved values, origins and relations. Recipes ride
// along so copy+bump preserves them; AlternativeName/SubType follow K1.
type EffectiveConfig struct {
	ThingID         string              `json:"thingId,omitempty"`
	Name            string              `json:"name,omitempty"`
	AlternativeName string              `json:"alternativeName,omitempty"`
	SubType         string              `json:"subType,omitempty"`
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
	Recipes         []Recipe            `json:"recipes,omitempty"`
	Relations       []RelationRef       `json:"relations,omitempty"`
}

// String encodes the box as "minLon,minLat,maxLon,maxLat" for ?bbox=.
func (b BBox) String() string {
	return itoaFloat(b.MinLon) + "," + itoaFloat(b.MinLat) + "," + itoaFloat(b.MaxLon) + "," + itoaFloat(b.MaxLat)
}

// String encodes the filter as "lon,lat,radiusMeters" for ?near=.
func (n NearFilter) String() string {
	return itoaFloat(n.Lon) + "," + itoaFloat(n.Lat) + "," + itoaFloat(n.RadiusMeters)
}

func itoaFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
