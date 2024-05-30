package application

import (
	"context"
	"time"
)

type SensorService interface {
	GetSensor(ctx context.Context, id string) (Sensor, error)
	GetSensors(ctx context.Context, offset, limit int) (SensorResult, error)
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type Type struct {
	URN  string `json:"urn"`
	Name string `json:"name"`
}

type DeviceProfile struct {
	Name     string `json:"name"`
	Decoder  string `json:"decoder"`
	Interval int    `json:"interval"`
}

type DeviceStatus struct {
	BatteryLevel int       `json:"batteryLevel"`
	ObservedAt   time.Time `json:"observedAt"`
}

type DeviceState struct {
	Online     bool      `json:"online"`
	State      int       `json:"state"`
	ObservedAt time.Time `json:"observedAt"`
}

type Sensor struct {
	Active        bool          `json:"active"`
	SensorID      string        `json:"sensorID"`
	DeviceID      string        `json:"deviceID"`
	Tenant        string        `json:"tenant"`
	Name          string        `json:"name"`
	Description   string        `json:"description"`
	Location      Location      `json:"location"`
	Environment   string        `json:"environment"`
	Types         []Type        `json:"types"`
	DeviceProfile DeviceProfile `json:"deviceProfile"`
	DeviceStatus  DeviceStatus  `json:"deviceStatus"`
	DeviceState   DeviceState   `json:"deviceState"`
}

type SensorResult struct {
	Sensors      []Sensor
	TotalRecords int
	Count        int
	Offset       int
	Limit        int
}
