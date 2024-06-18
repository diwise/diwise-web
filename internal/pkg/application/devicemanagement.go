package application

import (
	"context"
	"time"
)

type DeviceManagement interface {
	GetSensor(ctx context.Context, id string) (Sensor, error)
	GetSensors(ctx context.Context, offset, limit int) (SensorResult, error)
	UpdateSensor(ctx context.Context, deviceID string, fields map[string]any) error
	GetTenants(ctx context.Context) []string
	GetDeviceProfiles(ctx context.Context) []DeviceProfile
	GetStatistics(ctx context.Context) Statistics
}

type Statistics struct {
	Total    int `json:"total"`
	Active   int `json:"active"`
	Inactive int `json:"inactive"`
	Online   int `json:"online"`
	Unknown  int `json:"unknown"`
}

type Location struct {
	Latitude  float64 `json:"latitude,omitempty"`
	Longitude float64 `json:"longitude,omitempty"`
}

type Type struct {
	URN  string `json:"urn,omitempty"`
	Name string `json:"name"`
}

type DeviceProfile struct {
	Name     string    `json:"name"`
	Decoder  string    `json:"decoder,omitempty"`
	Interval int       `json:"interval,omitempty"`
	Types    *[]string `json:"types,omitempty"`
}

type DeviceStatus struct {
	BatteryLevel int       `json:"batteryLevel,omitempty"`
	ObservedAt   time.Time `json:"observedAt,omitempty"`
}

type DeviceState struct {
	Online     bool      `json:"online,omitempty"`
	State      int       `json:"state,omitempty"`
	ObservedAt time.Time `json:"observedAt,omitempty"`
}

type Sensor struct {
	Active        bool           `json:"active"`
	SensorID      string         `json:"sensorID,omitempty"`
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

type SensorResult struct {
	Sensors      []Sensor
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
