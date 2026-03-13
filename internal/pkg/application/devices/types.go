package devices

import (
	"context"
	"time"

	"github.com/diwise/diwise-web/internal/pkg/application/common"
)

type Management interface {
	GetDevice(ctx context.Context, id string) (Device, error)
	GetDevices(ctx context.Context, offset, limit int, args map[string][]string) (DeviceResult, error)
	UpdateDevice(ctx context.Context, deviceID string, fields map[string]any) error
	GetSensorStatus(ctx context.Context, id string) ([]SensorStatus, error)
	GetStatistics(ctx context.Context) (Statistics, error)
}

type Statistics struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Inactive int `json:"inactive"`
	Online   int `json:"online"`
	Unknown  int `json:"unknown"`
}

type Type struct {
	URN  string `json:"urn,omitempty"`
	Name string `json:"name"`
}

type SensorProfile struct {
	Name     string    `json:"name"`
	Decoder  string    `json:"decoder,omitempty"`
	Interval int       `json:"interval,omitempty"`
	Types    *[]string `json:"types,omitempty"`
}

type SensorStatus struct {
	BatteryLevel    int       `json:"batteryLevel,omitzero"`
	RSSI            *float64  `json:"rssi,omitempty"`
	LoRaSNR         *float64  `json:"loRaSNR,omitempty"`
	Frequency       *int64    `json:"frequency,omitempty"`
	SpreadingFactor *float64  `json:"spreadingFactor,omitempty"`
	DR              *int      `json:"dr,omitempty"`
	ObservedAt      time.Time `json:"observedAt"`
}

type DeviceState struct {
	Online     bool      `json:"online,omitempty"`
	State      int       `json:"state,omitempty"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
}

type Device struct {
	SensorID string `json:"sensorID,omitzero"`
	DeviceID string `json:"deviceID"`

	Active      bool            `json:"active"`
	Name        string          `json:"name,omitzero"`
	Description string          `json:"description,omitzero"`
	Location    common.Location `json:"location"`
	Environment *string         `json:"environment,omitzero"`
	Source      string          `json:"source,omitzero"`
	Tenant      string          `json:"tenant"`

	Interval    int          `json:"interval,omitzero"`
	DeviceState *DeviceState `json:"deviceState,omitempty"`

	Types    []Type            `json:"types,omitempty"`
	Metadata []common.Metadata `json:"metadata,omitempty"`

	SensorProfile *SensorProfile `json:"sensorProfile,omitempty"`
	SensorStatus  *SensorStatus  `json:"sensorStatus,omitempty"`

	Alarms []string `json:"alarms,omitempty"`
}

func (d Device) ObservedAt() time.Time {
	if d.SensorStatus != nil {
		return d.SensorStatus.ObservedAt
	}
	return time.Time{}
}

type DeviceResult struct {
	Devices      []Device
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
