package application

import (
	"context"
	"time"
)

type ThingManagement interface {
	NewThing(ctx context.Context, id string, fields map[string]any) error
	GetThing(ctx context.Context, id string) (Thing, error)
	GetThings(ctx context.Context, offset, limit int, parmas map[string][]string) (ThingResult, error)
	UpdateThing(ctx context.Context, thingID string, fields map[string]any) error
	GetTenants(ctx context.Context) []string
	GetTags(ctx context.Context) ([]string, error)
	GetTypes(ctx context.Context) ([]string, error)
	GetValidSensors(ctx context.Context, types []string) ([]SensorIdentifier, error)
	ConnectSensor(ctx context.Context, thingID, currentID, newID string) error
}

type Thing struct {
	ThingID      string         `json:"thing_id"`
	ID           string         `json:"id"`
	Type         string         `json:"type,omitempty"`
	Description  string         `json:"description,omitempty"`
	Location     Location       `json:"location,omitempty"`
	Measurements []Measurement  `json:"measurements,omitempty"`
	Name         string         `json:"name,omitempty"`
	Properties   map[string]any `json:"properties,omitempty"`
	Related      []Thing        `json:"related,omitempty"`
	Tags         []string       `json:"tags,omitempty"`
	Tenant       string         `json:"tenant,omitempty"`
}

func (t *Thing) AddProperties(props map[string]any) {
	delete(props, "thing_id")
	delete(props, "id")
	delete(props, "type")
	delete(props, "description")
	delete(props, "location")
	delete(props, "measurements")
	delete(props, "name")
	delete(props, "related")
	delete(props, "tags")
	delete(props, "tenant")

	t.Properties = props
}

type Measurement struct {
	ID          string    `json:"id"`
	Timestamp   time.Time `json:"timestamp"`
	Urn         string    `json:"urn"`
	BoolValue   *bool     `json:"vb,omitempty"`
	StringValue string    `json:"vs,omitempty"`
	Unit        string    `json:"unit,omitempty"`
	Value       *float64  `json:"v,omitempty"`
}

type SensorIdentifier struct {
	SensorID string `json:"sensorID,omitempty"`
	DeviceID string `json:"deviceID"`
	Decoder  string `json:"decoder"`
}

type ThingResult struct {
	Things       []Thing
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
