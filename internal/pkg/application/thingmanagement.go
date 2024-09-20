package application

import (
	"context"
	"time"
)

type ThingManagement interface {
	GetThing(ctx context.Context, id string) (Thing, error)
	GetThings(ctx context.Context, offset, limit int, parmas map[string][]string) (ThingResult, error)
	UpdateThing(ctx context.Context, thingID string, fields map[string]any) error
	GetTenants(ctx context.Context) []string
	GetTags(ctx context.Context) ([]string, error)
	GetTypes(ctx context.Context) ([]string, error)
	GetValidSensors(ctx context.Context, types []string) ([]string, error)
}

type Thing struct {
	ThingID      string        `json:"thing_id"`
	ID           string        `json:"id"`
	Type         string        `json:"type,omitempty"`
	Location     Location      `json:"location,omitempty"`
	Tenant       string        `json:"tenant,omitempty"`
	Tags         []string      `json:"tags,omitempty"`
	Measurements []Measurement `json:"measurements,omitempty"`
	Related      []Thing       `json:"related,omitempty"`
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

type ThingResult struct {
	Things       []Thing
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
