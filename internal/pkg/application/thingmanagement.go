package application

import (
	"context"
)

type ThingManagement interface {
	GetThing(ctx context.Context, id string) (Thing, error)
	GetThings(ctx context.Context, offset, limit int) (ThingResult, error)
}

type Thing struct {
	Active        bool           `json:"active"`
	ThingID       string         `json:"thingID,omitempty"`
	DeviceID      string         `json:"deviceID"`
	Tenant        string         `json:"tenant,omitempty"`
	Name          string         `json:"name,omitempty"`
	Description   string         `json:"description,omitempty"`
	Location      Location       `json:"location,omitempty"`
	Environment   *string        `json:"environment,omitempty"`
	Types         []Type         `json:"types,omitempty"`
	DeviceProfile *DeviceProfile `json:"deviceProfile,omitempty"`
	DeviceStatus  *DeviceStatus  `json:"deviceStatus,omitempty"`
	DeviceState   *DeviceState   `json:"deviceState,omitempty"`
}

type ThingResult struct {
	Things       []Thing
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
