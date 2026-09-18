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
}

// TemplateSpec wraps a template with parameters and recipes.
type TemplateSpec struct {
	Template Template `json:"template"`
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
