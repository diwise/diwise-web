package application

import (
	"context"
)

type ThingManagement interface {
	GetThing(ctx context.Context, id string) (Thing, error)
	GetThings(ctx context.Context, offset, limit int) (ThingResult, error)
}

type Thing struct {
	ThingID       string         `json:"thingID"`	
	ID            string         `json:"id"`	
	Type          string         `json:"type,omitempty"`
	Location      Location       `json:"location,omitempty"`
	Tenant        string         `json:"tenant,omitempty"`	
}

type ThingResult struct {
	Things       []Thing
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
