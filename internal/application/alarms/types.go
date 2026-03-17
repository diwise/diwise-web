package alarms

import (
	"context"
	"time"
)

type Management interface {
	GetAlarms(ctx context.Context, offset, limit int, args map[string][]string) (Result, error)
}

type Alarm struct {
	DeviceID   string    `json:"deviceID"`
	ObservedAt time.Time `json:"observedAt"`
	Types      []string  `json:"alarms"`
}

type Result struct {
	Alarms       []Alarm
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
